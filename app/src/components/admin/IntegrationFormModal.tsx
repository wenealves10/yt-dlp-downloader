import React, { useEffect, useMemo, useState } from "react";
import { Loader, X } from "lucide-react";
import { useCreateIntegration, useUpdateIntegration } from "../../hooks/useIntegrations";
import type { Integration } from "../../interface/Integration";
import { SegredoRevelado } from "./SegredoRevelado";

interface Props {
  open: boolean;
  /** Ausente cria; presente edita. */
  integracao?: Integration | null;
  onClose: () => void;
}

// Valores padrão pensados para "funciona e não derruba nada": uma integração
// nova nasce contida, e afrouxar é uma decisão explícita de quem a cadastra.
const PADROES = {
  daily_limit: 50,
  rate_limit_per_minute: 60,
  max_concurrent_downloads: 3,
  max_file_size_mb: 0,
};

export const IntegrationFormModal: React.FC<Props> = ({
  open,
  integracao,
  onClose,
}) => {
  const editando = !!integracao;
  const criar = useCreateIntegration();
  const atualizar = useUpdateIntegration();

  const [nome, setNome] = useState("");
  const [descricao, setDescricao] = useState("");
  const [urlBase, setUrlBase] = useState("");
  const [cota, setCota] = useState(PADROES.daily_limit);
  const [rpm, setRpm] = useState(PADROES.rate_limit_per_minute);
  const [simultaneos, setSimultaneos] = useState(PADROES.max_concurrent_downloads);
  const [tamanhoMB, setTamanhoMB] = useState(PADROES.max_file_size_mb);
  const [ips, setIps] = useState("");
  const [ativa, setAtiva] = useState(true);
  const [erro, setErro] = useState("");
  // A chave em claro da integração recém-criada. Enquanto ela estiver aqui, o
  // modal NÃO fecha sozinho: fechar levaria o único valor embora.
  const [chaveNova, setChaveNova] = useState("");

  useEffect(() => {
    if (!open) return;

    setErro("");
    setChaveNova("");

    if (integracao) {
      setNome(integracao.name);
      setDescricao(integracao.description || "");
      setUrlBase(integracao.callback_base_url || "");
      setCota(integracao.daily_limit);
      setRpm(integracao.rate_limit_per_minute);
      setSimultaneos(integracao.max_concurrent_downloads);
      setTamanhoMB(Math.round(integracao.max_file_size_bytes / (1024 * 1024)));
      setIps((integracao.allowed_ips || []).join("\n"));
      setAtiva(integracao.active);
      return;
    }

    setNome("");
    setDescricao("");
    setUrlBase("");
    setCota(PADROES.daily_limit);
    setRpm(PADROES.rate_limit_per_minute);
    setSimultaneos(PADROES.max_concurrent_downloads);
    setTamanhoMB(PADROES.max_file_size_mb);
    setIps("");
    setAtiva(true);
  }, [open, integracao]);

  const salvando = criar.isPending || atualizar.isPending;

  // Uma linha por IP é mais fácil de revisar que uma lista separada por vírgula
  // — e é o formato em que as listas de firewall costumam ser colecionadas.
  const listaIps = useMemo(
    () =>
      ips
        .split(/[\n,;]+/)
        .map((entrada) => entrada.trim())
        .filter(Boolean),
    [ips]
  );

  const enviar = (evento: React.FormEvent) => {
    evento.preventDefault();
    setErro("");

    const comum = {
      name: nome.trim(),
      description: descricao.trim() || undefined,
      callback_base_url: urlBase.trim() || undefined,
      daily_limit: cota,
      rate_limit_per_minute: rpm,
      max_concurrent_downloads: simultaneos,
      max_file_size_bytes: Math.max(0, tamanhoMB) * 1024 * 1024,
      allowed_ips: listaIps,
    };

    if (editando && integracao) {
      atualizar.mutate(
        { id: integracao.id, ...comum, active: ativa },
        { onSuccess: () => onClose(), onError: (falha) => setErro(falha.message) }
      );
      return;
    }

    criar.mutate(comum, {
      onSuccess: (resposta) => {
        // Não fecha: a chave precisa ser copiada antes.
        setChaveNova(resposta.api_key_plaintext || "");
        if (resposta.warning) setErro(resposta.warning);
      },
      onError: (falha) => setErro(falha.message),
    });
  };

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-black/70 p-4 sm:items-center">
      <div className="w-full max-w-2xl rounded-xl border border-gray-700 bg-gray-800 shadow-xl">
        <header className="flex items-center justify-between border-b border-gray-700 px-5 py-4">
          <div>
            <h2 className="text-lg font-semibold text-gray-100">
              {chaveNova
                ? "Integração criada"
                : editando
                  ? "Editar integração"
                  : "Nova integração"}
            </h2>
            <p className="mt-0.5 text-xs text-gray-400">
              {chaveNova
                ? "Guarde a chave antes de fechar esta janela."
                : "Um sistema que vai consumir a API de downloads."}
            </p>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded-lg p-2 text-gray-400 transition-colors hover:bg-gray-700 hover:text-gray-200"
            aria-label="Fechar"
          >
            <X size={18} />
          </button>
        </header>

        {chaveNova ? (
          <div className="space-y-4 p-5">
            <SegredoRevelado
              titulo="Chave de API"
              valor={chaveNova}
              unicaVez
              descricao={
                <>
                  O sistema integrado envia esta chave no header{" "}
                  <code className="text-gray-400">Authorization: Bearer …</code>.
                </>
              }
            />
            <p className="text-sm text-gray-400">
              Próximo passo: na aba <strong className="text-gray-200">Webhooks</strong>{" "}
              da integração, cadastre a URL que vai receber as notificações de
              download. Sem ela, o sistema integrado precisa consultar a rota de
              downloads para saber quando o arquivo ficou pronto.
            </p>
            {erro && (
              <p className="rounded-lg border border-amber-700 bg-amber-950/40 p-3 text-sm text-amber-200">
                {erro}
              </p>
            )}
            <div className="flex justify-end">
              <button
                type="button"
                onClick={onClose}
                className="rounded-lg bg-red-600 px-4 py-2 font-medium text-white transition-colors hover:bg-red-700"
              >
                Já copiei a chave
              </button>
            </div>
          </div>
        ) : (
          <form onSubmit={enviar} className="space-y-4 p-5">
            <div className="grid gap-4 sm:grid-cols-2">
              <label className="block sm:col-span-2">
                <span className="text-sm text-gray-300">Nome do sistema</span>
                <input
                  required
                  minLength={2}
                  maxLength={120}
                  value={nome}
                  onChange={(evento) => setNome(evento.target.value)}
                  placeholder="CRM Comercial"
                  className="mt-1 w-full rounded-lg border border-gray-700 bg-gray-900 px-3 py-2 text-sm text-gray-100 placeholder-gray-500 focus:border-red-500 focus:ring-2 focus:ring-red-500"
                />
              </label>

              <label className="block sm:col-span-2">
                <span className="text-sm text-gray-300">Descrição</span>
                <input
                  maxLength={500}
                  value={descricao}
                  onChange={(evento) => setDescricao(evento.target.value)}
                  placeholder="Para que este sistema usa os downloads"
                  className="mt-1 w-full rounded-lg border border-gray-700 bg-gray-900 px-3 py-2 text-sm text-gray-100 placeholder-gray-500 focus:border-red-500 focus:ring-2 focus:ring-red-500"
                />
              </label>

              <label className="block sm:col-span-2">
                <span className="text-sm text-gray-300">URL base do sistema</span>
                <input
                  type="url"
                  maxLength={500}
                  value={urlBase}
                  onChange={(evento) => setUrlBase(evento.target.value)}
                  placeholder="https://crm.suaempresa.com"
                  className="mt-1 w-full rounded-lg border border-gray-700 bg-gray-900 px-3 py-2 text-sm text-gray-100 placeholder-gray-500 focus:border-red-500 focus:ring-2 focus:ring-red-500"
                />
                <span className="mt-1 block text-xs text-gray-500">
                  Só referência, para identificar de onde vem o tráfego. As
                  notificações vão para a URL cadastrada na aba Webhooks.
                </span>
              </label>

              <label className="block">
                <span className="text-sm text-gray-300">Downloads por dia</span>
                <input
                  type="number"
                  min={0}
                  max={100000}
                  value={cota}
                  onChange={(evento) => setCota(Number(evento.target.value))}
                  className="mt-1 w-full rounded-lg border border-gray-700 bg-gray-900 px-3 py-2 text-sm text-gray-100 focus:border-red-500 focus:ring-2 focus:ring-red-500"
                />
                <span className="mt-1 block text-xs text-gray-500">
                  Reseta à meia-noite (São Paulo). 0 = sem teto.
                </span>
              </label>

              <label className="block">
                <span className="text-sm text-gray-300">Downloads simultâneos</span>
                <input
                  type="number"
                  min={0}
                  max={1000}
                  value={simultaneos}
                  onChange={(evento) => setSimultaneos(Number(evento.target.value))}
                  className="mt-1 w-full rounded-lg border border-gray-700 bg-gray-900 px-3 py-2 text-sm text-gray-100 focus:border-red-500 focus:ring-2 focus:ring-red-500"
                />
                <span className="mt-1 block text-xs text-gray-500">
                  Impede gastar a cota do dia toda de uma vez. 0 = sem teto.
                </span>
              </label>

              <label className="block">
                <span className="text-sm text-gray-300">Chamadas por minuto</span>
                <input
                  type="number"
                  min={1}
                  max={10000}
                  value={rpm}
                  onChange={(evento) => setRpm(Number(evento.target.value))}
                  className="mt-1 w-full rounded-lg border border-gray-700 bg-gray-900 px-3 py-2 text-sm text-gray-100 focus:border-red-500 focus:ring-2 focus:ring-red-500"
                />
                <span className="mt-1 block text-xs text-gray-500">
                  Por chave de API, não por integração.
                </span>
              </label>

              <label className="block">
                <span className="text-sm text-gray-300">Tamanho máximo (MB)</span>
                <input
                  type="number"
                  min={0}
                  value={tamanhoMB}
                  onChange={(evento) => setTamanhoMB(Number(evento.target.value))}
                  className="mt-1 w-full rounded-lg border border-gray-700 bg-gray-900 px-3 py-2 text-sm text-gray-100 focus:border-red-500 focus:ring-2 focus:ring-red-500"
                />
                <span className="mt-1 block text-xs text-gray-500">
                  Por arquivo. 0 = sem teto.
                </span>
              </label>

              <label className="block sm:col-span-2">
                <span className="text-sm text-gray-300">IPs autorizados</span>
                <textarea
                  rows={3}
                  value={ips}
                  onChange={(evento) => setIps(evento.target.value)}
                  placeholder={"203.0.113.10\n198.51.100.0/24"}
                  className="mt-1 w-full rounded-lg border border-gray-700 bg-gray-900 px-3 py-2 font-mono text-xs text-gray-100 placeholder-gray-600 focus:border-red-500 focus:ring-2 focus:ring-red-500"
                />
                <span className="mt-1 block text-xs text-gray-500">
                  Um por linha, IP ou CIDR.{" "}
                  <strong className="text-gray-400">
                    Vazio aceita qualquer origem
                  </strong>{" "}
                  — preencher é a principal defesa caso a chave vaze.
                </span>
              </label>
            </div>

            {editando && (
              <label className="flex items-center gap-2 text-sm text-gray-300">
                <input
                  type="checkbox"
                  checked={ativa}
                  onChange={(evento) => setAtiva(evento.target.checked)}
                  className="h-4 w-4 rounded border-gray-600 bg-gray-900 text-red-600 focus:ring-red-500"
                />
                Integração ativa
                <span className="text-xs text-gray-500">
                  (desativada, toda chamada dela é recusada)
                </span>
              </label>
            )}

            {erro && (
              <p className="rounded-lg border border-red-700 bg-red-900/30 p-3 text-sm text-red-200">
                {erro}
              </p>
            )}

            <div className="flex justify-end gap-2 border-t border-gray-700 pt-4">
              <button
                type="button"
                onClick={onClose}
                className="rounded-lg px-4 py-2 text-sm text-gray-300 transition-colors hover:bg-gray-700"
              >
                Cancelar
              </button>
              <button
                type="submit"
                disabled={salvando}
                className="flex items-center gap-2 rounded-lg bg-red-600 px-4 py-2 font-medium text-white transition-colors hover:bg-red-700 disabled:opacity-60"
              >
                {salvando && <Loader size={16} className="animate-spin" />}
                {editando ? "Salvar" : "Criar integração"}
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  );
};
