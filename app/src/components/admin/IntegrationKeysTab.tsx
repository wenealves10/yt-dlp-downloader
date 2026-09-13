import React, { useState } from "react";
import { KeyRound, Loader, Plus, RefreshCw, ShieldOff } from "lucide-react";
import { useCreateKey, useIntegrationKeys, useRevokeKey } from "../../hooks/useIntegrations";
import { formatDateTime } from "./format";
import { SegredoRevelado } from "./SegredoRevelado";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import type { ApiKey } from "../../interface/Integration";

interface Props {
  integrationId: string;
}

export const IntegrationKeysTab: React.FC<Props> = ({ integrationId }) => {
  const { data, isLoading } = useIntegrationKeys(integrationId);
  const criar = useCreateKey();
  const revogar = useRevokeKey();

  const [rotulo, setRotulo] = useState("");
  const [validade, setValidade] = useState("");
  const [chaveNova, setChaveNova] = useState("");
  const [erro, setErro] = useState("");
  const [revogando, setRevogando] = useState<ApiKey | null>(null);
  const [erroRevogacao, setErroRevogacao] = useState("");
  // Regerar substitui: a chave nova entra e as anteriores caem juntas.
  const [confirmandoRegeracao, setConfirmandoRegeracao] = useState(false);

  const emitir = (substituirAnteriores: boolean) => {
    setErro("");
    setChaveNova("");
    criar.mutate(
      {
        id: integrationId,
        label: rotulo.trim() || undefined,
        expires_in_days: validade ? Number(validade) : undefined,
        revoke_others: substituirAnteriores,
      },
      {
        onSuccess: (resposta) => {
          setChaveNova(resposta.api_key_plaintext);
          setRotulo("");
          setValidade("");
          setConfirmandoRegeracao(false);
        },
        onError: (falha) => {
          setErro(falha.message);
          setConfirmandoRegeracao(false);
        },
      }
    );
  };

  const confirmarRevogacao = () => {
    if (!revogando) return;
    setErroRevogacao("");
    revogar.mutate(
      { id: integrationId, keyId: revogando.id },
      {
        onSuccess: () => setRevogando(null),
        onError: (falha) => setErroRevogacao(falha.message),
      }
    );
  };

  const chaves = data?.api_keys ?? [];
  const ativas = chaves.filter((chave) => !chave.revoked);

  return (
    <div className="space-y-5">
      {chaveNova && (
        <SegredoRevelado
          titulo="Chave de API nova"
          valor={chaveNova}
          unicaVez
          descricao="Envie no header Authorization: Bearer … (ou X-API-Key)."
        />
      )}

      <section className="rounded-xl border border-gray-700 bg-gray-800/60 p-4">
        <h3 className="text-sm font-semibold text-gray-200">Emitir chave</h3>
        <p className="mt-1 text-xs text-gray-500">
          Duas chaves válidas ao mesmo tempo é como se troca a credencial de um
          sistema em produção sem derrubá-lo: sobe a nova, atualiza o cliente,
          revoga a velha. <strong className="text-gray-400">Regerar</strong> faz
          os dois passos de uma vez — a chave antiga para de funcionar na hora.
        </p>

        <div className="mt-3 flex flex-wrap items-end gap-2">
          <label className="block min-w-[180px] flex-1">
            <span className="text-xs text-gray-400">Identificação</span>
            <input
              value={rotulo}
              onChange={(evento) => setRotulo(evento.target.value)}
              placeholder="ex.: produção, homologação"
              maxLength={120}
              className="mt-1 w-full rounded-lg border border-gray-700 bg-gray-900 px-3 py-2 text-sm text-gray-100 placeholder-gray-600"
            />
          </label>
          <label className="block w-40">
            <span className="text-xs text-gray-400">Expira em (dias)</span>
            <input
              type="number"
              min={1}
              max={3650}
              value={validade}
              onChange={(evento) => setValidade(evento.target.value)}
              placeholder="sem validade"
              className="mt-1 w-full rounded-lg border border-gray-700 bg-gray-900 px-3 py-2 text-sm text-gray-100 placeholder-gray-600"
            />
          </label>
          <button
            type="button"
            onClick={() => emitir(false)}
            disabled={criar.isPending}
            className="flex items-center gap-2 rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-red-700 disabled:opacity-60"
          >
            {criar.isPending ? (
              <Loader size={16} className="animate-spin" />
            ) : (
              <Plus size={16} />
            )}
            Emitir
          </button>
          <button
            type="button"
            onClick={() => setConfirmandoRegeracao(true)}
            disabled={criar.isPending || ativas.length === 0}
            className="flex items-center gap-2 rounded-lg border border-gray-600 px-4 py-2 text-sm text-gray-300 transition-colors hover:bg-gray-700 disabled:opacity-40"
            title="Emite uma chave nova e revoga todas as atuais"
          >
            <RefreshCw size={16} />
            Regerar
          </button>
        </div>

        {erro && (
          <p className="mt-3 rounded-lg border border-red-700 bg-red-900/30 p-3 text-sm text-red-200">
            {erro}
          </p>
        )}
      </section>

      {isLoading ? (
        <div className="flex justify-center py-10">
          <Loader className="animate-spin text-gray-500" />
        </div>
      ) : chaves.length === 0 ? (
        <p className="rounded-xl border border-gray-700 bg-gray-800/40 p-6 text-center text-sm text-gray-400">
          Nenhuma chave emitida. Sem chave, o sistema integrado não consegue
          autenticar.
        </p>
      ) : (
        <div className="overflow-x-auto rounded-xl border border-gray-700">
          <table className="min-w-full divide-y divide-gray-700 text-sm">
            <thead className="bg-gray-800/80 text-xs uppercase tracking-wide text-gray-400">
              <tr>
                <th className="px-4 py-3 text-left">Chave</th>
                <th className="px-4 py-3 text-left">Identificação</th>
                <th className="px-4 py-3 text-left">Último uso</th>
                <th className="px-4 py-3 text-left">Origem</th>
                <th className="px-4 py-3 text-left">Situação</th>
                <th className="px-4 py-3 text-right">Ações</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-800 bg-gray-900/40">
              {chaves.map((chave) => (
                <tr
                  key={chave.id}
                  className={chave.revoked ? "opacity-60" : "hover:bg-gray-800/40"}
                >
                  <td className="px-4 py-3">
                    <span className="inline-flex items-center gap-2 font-mono text-xs text-gray-200">
                      <KeyRound size={14} className="text-gray-500" />
                      {chave.masked}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-gray-300">{chave.label}</td>
                  <td className="px-4 py-3 text-gray-400">
                    {formatDateTime(chave.last_used_at)}
                  </td>
                  <td className="px-4 py-3 font-mono text-xs text-gray-400">
                    {chave.last_used_ip || "—"}
                  </td>
                  <td className="px-4 py-3">
                    {chave.revoked ? (
                      <span
                        className="rounded bg-gray-700 px-2 py-0.5 text-xs text-gray-300"
                        title={chave.revoked_reason}
                      >
                        revogada {formatDateTime(chave.revoked_at)}
                      </span>
                    ) : chave.expires_at ? (
                      <span className="rounded border border-amber-700/50 bg-amber-600/20 px-2 py-0.5 text-xs text-amber-300">
                        expira {formatDateTime(chave.expires_at)}
                      </span>
                    ) : (
                      <span className="rounded border border-green-700/50 bg-green-600/20 px-2 py-0.5 text-xs text-green-300">
                        ativa
                      </span>
                    )}
                  </td>
                  <td className="px-4 py-3 text-right">
                    {!chave.revoked && (
                      <button
                        type="button"
                        onClick={() => setRevogando(chave)}
                        className="rounded-lg p-2 text-gray-400 transition-colors hover:bg-red-900/40 hover:text-red-300"
                        aria-label="Revogar"
                        title="Revogar"
                      >
                        <ShieldOff size={16} />
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <ConfirmDialog
        open={confirmandoRegeracao}
        titulo="Regerar a chave desta integração?"
        descricao={`A chave nova aparece na tela uma única vez e ${
          ativas.length > 1 ? `as ${ativas.length} chaves atuais` : "a chave atual"
        } deixam de funcionar imediatamente. O sistema integrado para de baixar até ser atualizado.`}
        confirmarLabel="Sim, regerar"
        processando={criar.isPending}
        onConfirm={() => emitir(true)}
        onCancel={() => setConfirmandoRegeracao(false)}
      />

      <ConfirmDialog
        open={!!revogando}
        titulo="Revogar esta chave?"
        alvo={revogando?.masked}
        descricao="Qualquer chamada que a use passa a ser recusada na hora. O registro da chave é preservado para auditoria."
        confirmarLabel="Sim, revogar"
        processando={revogar.isPending}
        erro={erroRevogacao}
        onConfirm={confirmarRevogacao}
        onCancel={() => {
          setRevogando(null);
          setErroRevogacao("");
        }}
      />
    </div>
  );
};
