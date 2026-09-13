import React, { useState } from "react";
import { Link } from "react-router-dom";
import {
  ChevronLeft,
  ChevronRight,
  KeyRound,
  Loader,
  Pencil,
  Plug,
  Plus,
  Search,
  Trash2,
} from "lucide-react";
import { AdminShell } from "./AdminShell";
import { IntegrationFormModal } from "./IntegrationFormModal";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import { useDeleteIntegration, useIntegrations } from "../../hooks/useIntegrations";
import { formatBytes, formatDateTime, formatNumber } from "./format";
import type { Integration } from "../../interface/Integration";

export const IntegrationsPage: React.FC = () => {
  const [page, setPage] = useState(1);
  const [busca, setBusca] = useState("");
  const [buscaAplicada, setBuscaAplicada] = useState("");
  const [status, setStatus] = useState("");
  const [criando, setCriando] = useState(false);
  const [editando, setEditando] = useState<Integration | null>(null);
  const [removendo, setRemovendo] = useState<Integration | null>(null);
  const [erroRemocao, setErroRemocao] = useState("");

  const filtros = { page, perPage: 20, search: buscaAplicada, status };
  const { data, isLoading, isError, isFetching } = useIntegrations(filtros);
  const remover = useDeleteIntegration();

  const aplicarBusca = (evento: React.FormEvent) => {
    evento.preventDefault();
    setPage(1);
    setBuscaAplicada(busca.trim());
  };

  const confirmarRemocao = () => {
    if (!removendo) return;
    setErroRemocao("");
    remover.mutate(removendo.id, {
      onSuccess: () => setRemovendo(null),
      onError: (falha) => setErroRemocao(falha.message),
    });
  };

  const integracoes = data?.integrations ?? [];

  return (
    <AdminShell
      titulo="Integrações"
      descricao="Sistemas que consomem a API de downloads, com chave, cota e auditoria próprias."
      acoes={
        <>
          <a
            href={`${import.meta.env.VITE_API_URL}/docs`}
            target="_blank"
            rel="noreferrer"
            className="rounded-lg border border-gray-700 px-4 py-2 text-sm text-gray-300 transition-colors hover:bg-gray-800"
          >
            Documentação da API
          </a>
          <button
            type="button"
            onClick={() => setCriando(true)}
            className="flex items-center gap-2 rounded-lg bg-red-600 px-4 py-2 font-medium text-white transition-colors hover:bg-red-700"
          >
            <Plus size={16} />
            Nova integração
          </button>
        </>
      }
    >
      <div className="mb-4 flex flex-wrap items-center gap-2">
        <form onSubmit={aplicarBusca} className="relative min-w-[220px] flex-1">
          <Search
            size={16}
            className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500"
          />
          <input
            type="search"
            value={busca}
            onChange={(evento) => setBusca(evento.target.value)}
            placeholder="Buscar por nome ou descrição..."
            className="w-full rounded-lg border border-gray-700 bg-gray-800 py-2 pl-9 pr-3 text-sm placeholder-gray-500 transition-all focus:border-red-500 focus:ring-2 focus:ring-red-500"
          />
        </form>

        <select
          value={status}
          onChange={(evento) => {
            setPage(1);
            setStatus(evento.target.value);
          }}
          className="rounded-lg border border-gray-700 bg-gray-800 px-3 py-2 text-sm text-gray-200"
        >
          <option value="">Todas</option>
          <option value="active">Ativas</option>
          <option value="blocked">Desativadas</option>
        </select>

        {isFetching && <Loader size={16} className="animate-spin text-gray-500" />}
      </div>

      {isLoading ? (
        <div className="flex justify-center py-16">
          <Loader className="animate-spin text-gray-500" />
        </div>
      ) : isError ? (
        <p className="rounded-xl border border-red-800 bg-red-900/20 p-4 text-sm text-red-200">
          Não foi possível carregar as integrações.
        </p>
      ) : integracoes.length === 0 ? (
        <div className="rounded-xl border border-gray-700 bg-gray-800/40 p-10 text-center">
          <Plug size={32} className="mx-auto text-gray-600" />
          <p className="mt-3 text-gray-300">Nenhuma integração cadastrada.</p>
          <p className="mx-auto mt-1 max-w-md text-sm text-gray-500">
            Cada integração é uma conta de sistema: ela recebe uma chave de API,
            uma cota diária própria e aparece separada dos usuários no painel.
          </p>
        </div>
      ) : (
        <div className="overflow-x-auto rounded-xl border border-gray-700">
          <table className="min-w-full divide-y divide-gray-700 text-sm">
            <thead className="bg-gray-800/80 text-xs uppercase tracking-wide text-gray-400">
              <tr>
                <th className="px-4 py-3 text-left">Sistema</th>
                <th className="px-4 py-3 text-left">Hoje</th>
                <th className="px-4 py-3 text-left">Em andamento</th>
                <th className="px-4 py-3 text-left">Total</th>
                <th className="px-4 py-3 text-left">Armazenado</th>
                <th className="px-4 py-3 text-left">Chaves</th>
                <th className="px-4 py-3 text-left">Último uso</th>
                <th className="px-4 py-3 text-right">Ações</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-800 bg-gray-900/40">
              {integracoes.map((item) => (
                <tr key={item.id} className="hover:bg-gray-800/40">
                  <td className="px-4 py-3">
                    <Link
                      to={`/admin/integrations/${item.id}`}
                      className="font-medium text-gray-100 hover:text-red-400"
                    >
                      {item.name}
                    </Link>
                    <div className="mt-0.5 flex items-center gap-2">
                      {!item.active && (
                        <span className="rounded bg-gray-700 px-1.5 py-0.5 text-[11px] text-gray-300">
                          desativada
                        </span>
                      )}
                      {item.allowed_ips.length > 0 ? (
                        <span className="text-[11px] text-gray-500">
                          {item.allowed_ips.length} IP(s) autorizado(s)
                        </span>
                      ) : (
                        // Vale destacar: é a diferença entre uma chave vazada
                        // ser inútil e ser utilizável de qualquer lugar.
                        <span className="text-[11px] text-amber-500/80">
                          qualquer IP
                        </span>
                      )}
                    </div>
                  </td>
                  <td className="px-4 py-3 tabular-nums text-gray-300">
                    {formatNumber(item.downloads_today)}
                    <span className="text-gray-600">
                      {item.daily_limit > 0 ? ` / ${formatNumber(item.daily_limit)}` : " / ∞"}
                    </span>
                  </td>
                  <td className="px-4 py-3 tabular-nums text-gray-300">
                    {item.downloads_active > 0 ? (
                      <span className="text-blue-300">
                        {formatNumber(item.downloads_active)}
                      </span>
                    ) : (
                      <span className="text-gray-600">—</span>
                    )}
                  </td>
                  <td className="px-4 py-3 tabular-nums text-gray-300">
                    {formatNumber(item.downloads_total)}
                  </td>
                  <td className="px-4 py-3 tabular-nums text-gray-300">
                    {formatBytes(item.storage_bytes)}
                  </td>
                  <td className="px-4 py-3">
                    <span className="inline-flex items-center gap-1 text-gray-300">
                      <KeyRound size={14} className="text-gray-500" />
                      {item.active_keys}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-gray-400">
                    {formatDateTime(item.last_used_at)}
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex justify-end gap-1">
                      <button
                        type="button"
                        onClick={() => setEditando(item)}
                        className="rounded-lg p-2 text-gray-400 transition-colors hover:bg-gray-700 hover:text-gray-200"
                        aria-label="Editar"
                        title="Editar"
                      >
                        <Pencil size={16} />
                      </button>
                      <button
                        type="button"
                        onClick={() => setRemovendo(item)}
                        className="rounded-lg p-2 text-gray-400 transition-colors hover:bg-red-900/40 hover:text-red-300"
                        aria-label="Remover"
                        title="Remover"
                      >
                        <Trash2 size={16} />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {data && data.total > data.per_page && (
        <div className="mt-4 flex items-center justify-between text-sm text-gray-400">
          <span>
            Página {data.page} — {formatNumber(data.total)} integração(ões)
          </span>
          <div className="flex gap-2">
            <button
              type="button"
              disabled={!data.prev_page}
              onClick={() => setPage((atual) => Math.max(1, atual - 1))}
              className="flex items-center gap-1 rounded-lg border border-gray-700 px-3 py-1.5 transition-colors hover:bg-gray-800 disabled:opacity-40"
            >
              <ChevronLeft size={16} />
              Anterior
            </button>
            <button
              type="button"
              disabled={!data.next_page}
              onClick={() => setPage((atual) => atual + 1)}
              className="flex items-center gap-1 rounded-lg border border-gray-700 px-3 py-1.5 transition-colors hover:bg-gray-800 disabled:opacity-40"
            >
              Próxima
              <ChevronRight size={16} />
            </button>
          </div>
        </div>
      )}

      <IntegrationFormModal open={criando} onClose={() => setCriando(false)} />
      <IntegrationFormModal
        open={!!editando}
        integracao={editando}
        onClose={() => setEditando(null)}
      />

      <ConfirmDialog
        open={!!removendo}
        titulo="Remover esta integração?"
        alvo={removendo?.name}
        descricao="Todas as chaves de API dela são revogadas na hora, e o sistema integrado para de conseguir baixar. O histórico de downloads é preservado."
        processando={remover.isPending}
        erro={erroRemocao}
        onConfirm={confirmarRemocao}
        onCancel={() => {
          setRemovendo(null);
          setErroRemocao("");
        }}
      />
    </AdminShell>
  );
};
