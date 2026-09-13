import React, { useState } from "react";
import { Loader, Search } from "lucide-react";
import { useIntegrationDownloads } from "../../hooks/useIntegrations";
import { formatBytes, formatDateTime } from "./format";
import { Paginacao } from "./IntegrationLogsTab";

interface Props {
  integrationId: string;
}

const CORES_STATUS: Record<string, string> = {
  PENDING: "bg-gray-700 text-gray-300",
  PROCESSING: "bg-blue-600/20 text-blue-300 border border-blue-700/50",
  COMPLETED: "bg-green-600/20 text-green-300 border border-green-700/50",
  FAILED: "bg-red-600/20 text-red-300 border border-red-700/50",
  CANCELED: "bg-gray-700 text-gray-400",
  EXPIRED: "bg-amber-600/20 text-amber-300 border border-amber-700/50",
  RETRYING: "bg-blue-600/20 text-blue-300 border border-blue-700/50",
};

export const IntegrationDownloadsTab: React.FC<Props> = ({ integrationId }) => {
  const [page, setPage] = useState(1);
  const [status, setStatus] = useState("");
  const [busca, setBusca] = useState("");
  const [buscaAplicada, setBuscaAplicada] = useState("");

  const { data, isLoading } = useIntegrationDownloads(integrationId, {
    page,
    perPage: 20,
    status,
    search: buscaAplicada,
  });

  const downloads = data?.downloads ?? [];

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-2">
        <form
          onSubmit={(evento) => {
            evento.preventDefault();
            setPage(1);
            setBuscaAplicada(busca.trim());
          }}
          className="relative min-w-[220px] flex-1"
        >
          <Search
            size={16}
            className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500"
          />
          <input
            type="search"
            value={busca}
            onChange={(evento) => setBusca(evento.target.value)}
            placeholder="Buscar por título ou URL..."
            className="w-full rounded-lg border border-gray-700 bg-gray-800 py-2 pl-9 pr-3 text-sm placeholder-gray-500"
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
          <option value="">Todos os status</option>
          <option value="PENDING">Na fila</option>
          <option value="PROCESSING">Em andamento</option>
          <option value="COMPLETED">Concluídos</option>
          <option value="FAILED">Falhados</option>
          <option value="CANCELED">Cancelados</option>
          <option value="EXPIRED">Expirados</option>
        </select>
      </div>

      {isLoading ? (
        <div className="flex justify-center py-10">
          <Loader className="animate-spin text-gray-500" />
        </div>
      ) : downloads.length === 0 ? (
        <p className="rounded-xl border border-gray-700 bg-gray-800/40 p-6 text-center text-sm text-gray-400">
          Nenhum download desta integração com estes filtros.
        </p>
      ) : (
        <div className="overflow-x-auto rounded-xl border border-gray-700">
          <table className="min-w-full divide-y divide-gray-700 text-sm">
            <thead className="bg-gray-800/80 text-xs uppercase tracking-wide text-gray-400">
              <tr>
                <th className="px-4 py-3 text-left">Conteúdo</th>
                <th className="px-4 py-3 text-left">Status</th>
                <th className="px-4 py-3 text-left">Plataforma</th>
                <th className="px-4 py-3 text-left">Qualidade</th>
                <th className="px-4 py-3 text-left">Tamanho</th>
                <th className="px-4 py-3 text-left">Criado</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-800 bg-gray-900/40">
              {downloads.map((download) => (
                <tr key={download.id} className="hover:bg-gray-800/40">
                  <td className="max-w-md px-4 py-3">
                    <p className="truncate text-gray-100" title={download.title}>
                      {download.title}
                    </p>
                    <a
                      href={download.original_url}
                      target="_blank"
                      rel="noreferrer"
                      className="block truncate text-xs text-gray-500 hover:text-gray-300"
                      title={download.original_url}
                    >
                      {download.original_url}
                    </a>
                    {/* O detalhe técnico só existe nesta tela: é o que evita
                        entrar no container para descobrir por que falhou. */}
                    {download.error_detail && (
                      <p className="mt-1 break-words rounded bg-red-950/30 p-1.5 font-mono text-[11px] text-red-300">
                        {download.error_detail}
                      </p>
                    )}
                    {!download.error_detail && download.error_message && (
                      <p className="mt-1 text-[11px] text-red-300/80">
                        {download.error_message}
                      </p>
                    )}
                  </td>
                  <td className="px-4 py-3">
                    <span
                      className={`rounded px-2 py-0.5 text-[11px] ${
                        CORES_STATUS[download.status] || "bg-gray-700 text-gray-300"
                      }`}
                    >
                      {download.status}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-gray-300">
                    {download.platform_label || download.platform || "—"}
                  </td>
                  <td className="px-4 py-3 text-gray-400">
                    {download.quality_label || download.format}
                  </td>
                  <td className="px-4 py-3 tabular-nums text-gray-300">
                    {download.file_size_bytes
                      ? formatBytes(download.file_size_bytes)
                      : "—"}
                  </td>
                  <td className="whitespace-nowrap px-4 py-3 text-xs text-gray-400">
                    {formatDateTime(download.created_at)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <Paginacao
        page={data?.page ?? 1}
        total={data?.total ?? 0}
        perPage={data?.per_page ?? 20}
        temAnterior={!!data?.prev_page}
        temProxima={!!data?.next_page}
        onPage={setPage}
        rotulo="download(s)"
      />
    </div>
  );
};
