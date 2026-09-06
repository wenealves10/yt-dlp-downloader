import React, { useState } from "react";
import { useSearchParams } from "react-router-dom";
import {
  ChevronLeft,
  ChevronRight,
  Loader,
  Search,
  Trash2,
  X,
} from "lucide-react";
import { AdminShell } from "./AdminShell";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import { useAdminDownloads, useDeleteDownload } from "../../hooks/useAdmin";
import {
  formatBytes,
  formatDateTime,
  formatDuration,
  formatNumber,
} from "./format";
import type { AdminDownload } from "../../interface/Admin";

const CORES_STATUS: Record<string, string> = {
  COMPLETED: "bg-green-600/20 text-green-300 border border-green-700/50",
  PROCESSING: "bg-blue-600/20 text-blue-300 border border-blue-700/50",
  PENDING: "bg-gray-700 text-gray-300",
  RETRYING: "bg-blue-600/20 text-blue-300 border border-blue-700/50",
  FAILED: "bg-red-600/20 text-red-300 border border-red-700/50",
  EXPIRED: "bg-amber-600/20 text-amber-300 border border-amber-700/50",
  CANCELED: "bg-gray-700 text-gray-400",
};

export const DownloadsPage: React.FC = () => {
  // O filtro por usuário chega pela URL, vindo da tabela de usuários — assim o
  // link é compartilhável e o botão "voltar" do navegador funciona.
  const [searchParams, setSearchParams] = useSearchParams();
  const usuarioFiltrado = searchParams.get("user_id") ?? "";

  const [page, setPage] = useState(1);
  const [busca, setBusca] = useState("");
  const [buscaAplicada, setBuscaAplicada] = useState("");
  const [status, setStatus] = useState("");
  const [removendo, setRemovendo] = useState<AdminDownload | null>(null);
  const [erroRemocao, setErroRemocao] = useState("");

  const { data, isLoading, isError, isFetching } = useAdminDownloads({
    page,
    perPage: 20,
    search: buscaAplicada,
    status,
    user_id: usuarioFiltrado,
  });
  const remover = useDeleteDownload();

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

  const cancelarRemocao = () => {
    setRemovendo(null);
    setErroRemocao("");
  };

  const limparUsuario = () => {
    searchParams.delete("user_id");
    setSearchParams(searchParams);
    setPage(1);
  };

  const emailFiltrado = data?.downloads[0]?.user_email;

  return (
    <AdminShell
      titulo="Downloads"
      descricao="Histórico completo da plataforma, de todos os usuários."
    >
      <div className="mb-4 flex flex-wrap items-center gap-2">
        <form onSubmit={aplicarBusca} className="relative flex-1 min-w-[220px]">
          <Search
            size={16}
            className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500"
          />
          <input
            type="search"
            value={busca}
            onChange={(evento) => setBusca(evento.target.value)}
            placeholder="Buscar por título ou e-mail..."
            className="w-full bg-gray-800 border border-gray-700 rounded-lg py-2 pl-9 pr-3 text-sm placeholder-gray-500 focus:ring-2 focus:ring-red-500 focus:border-red-500 transition-all"
          />
        </form>

        <select
          value={status}
          onChange={(evento) => {
            setPage(1);
            setStatus(evento.target.value);
          }}
          className="bg-gray-800 border border-gray-700 rounded-lg py-2 px-3 text-sm text-gray-200"
        >
          <option value="">Todos os status</option>
          <option value="COMPLETED">Concluídos</option>
          <option value="PROCESSING">Processando</option>
          <option value="PENDING">Na fila</option>
          <option value="FAILED">Falhas</option>
          <option value="EXPIRED">Expirados</option>
        </select>

        {usuarioFiltrado && (
          <button
            type="button"
            onClick={limparUsuario}
            className="flex items-center gap-1.5 px-3 py-2 rounded-lg bg-gray-800 border border-gray-600 text-sm text-gray-300 hover:bg-gray-700 transition-colors"
          >
            <span className="text-gray-500">usuário:</span>
            {emailFiltrado ?? "filtrado"}
            <X size={14} />
          </button>
        )}
      </div>

      {isLoading && (
        <p className="flex items-center gap-2 text-gray-400">
          <Loader className="animate-spin" size={18} /> Carregando downloads...
        </p>
      )}

      {isError && !isLoading && (
        <p className="text-red-400">Não foi possível carregar os downloads.</p>
      )}

      {data && (
        <>
          <div className="bg-gray-800 border border-gray-700 rounded-xl overflow-hidden">
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead className="bg-gray-800/80 border-b border-gray-700">
                  <tr className="text-left text-xs text-gray-400 uppercase tracking-wide">
                    <th className="px-4 py-3 font-medium">Título</th>
                    <th className="px-4 py-3 font-medium">Usuário</th>
                    <th className="px-4 py-3 font-medium">Status</th>
                    <th className="px-4 py-3 font-medium">Formato</th>
                    <th className="px-4 py-3 font-medium text-right">Tamanho</th>
                    <th className="px-4 py-3 font-medium">Criado</th>
                    <th className="px-4 py-3 font-medium text-right">Ações</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-700">
                  {data.downloads.map((download) => (
                    <tr
                      key={download.id}
                      className="hover:bg-gray-700/30 transition-colors"
                    >
                      <td className="px-4 py-3 max-w-xs">
                        <p className="text-gray-100 truncate" title={download.title}>
                          {download.title || "—"}
                        </p>
                        <p className="text-xs text-gray-500">
                          {formatDuration(download.duration_seconds)}
                          {download.error_message && (
                            <span
                              className="text-red-400 ml-2"
                              title={download.error_message}
                            >
                              erro
                            </span>
                          )}
                        </p>
                      </td>
                      <td className="px-4 py-3">
                        <p className="text-gray-300 text-xs">{download.user_name}</p>
                        <p className="text-xs text-gray-500">{download.user_email}</p>
                      </td>
                      <td className="px-4 py-3">
                        <span
                          className={`inline-block px-2 py-0.5 rounded-full text-xs font-medium ${
                            CORES_STATUS[download.status] ?? CORES_STATUS.PENDING
                          }`}
                        >
                          {download.status}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-gray-400">{download.format}</td>
                      <td className="px-4 py-3 text-right text-gray-300 tabular-nums">
                        {download.file_size_bytes > 0
                          ? formatBytes(download.file_size_bytes)
                          : "—"}
                      </td>
                      <td className="px-4 py-3 text-gray-400 text-xs">
                        {formatDateTime(download.created_at)}
                      </td>
                      <td className="px-4 py-3 text-right">
                        <button
                          type="button"
                          onClick={() => setRemovendo(download)}
                          title="Remover"
                          className="p-2 rounded-md text-red-400 hover:bg-gray-700 transition-colors"
                        >
                          <Trash2 size={15} />
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            {data.downloads.length === 0 && (
              <p className="py-12 text-center text-sm text-gray-500">
                Nenhum download encontrado com esses filtros.
              </p>
            )}
          </div>

          <div className="mt-4 flex items-center justify-between gap-4 text-sm">
            <p className="text-gray-500">
              {formatNumber(data.total)} download(s)
              {isFetching && <span className="ml-2 text-gray-600">atualizando…</span>}
            </p>
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={() => setPage((atual) => Math.max(1, atual - 1))}
                disabled={!data.prev_page}
                className="flex items-center gap-1 px-3 py-1.5 rounded-lg border border-gray-700 text-gray-300 hover:bg-gray-800 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
              >
                <ChevronLeft size={16} />
                Anterior
              </button>
              <span className="text-gray-500">página {data.page}</span>
              <button
                type="button"
                onClick={() => setPage((atual) => atual + 1)}
                disabled={!data.next_page}
                className="flex items-center gap-1 px-3 py-1.5 rounded-lg border border-gray-700 text-gray-300 hover:bg-gray-800 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
              >
                Próxima
                <ChevronRight size={16} />
              </button>
            </div>
          </div>
        </>
      )}

      <ConfirmDialog
        open={removendo !== null}
        titulo="Remover este download?"
        alvo={removendo?.title || removendo?.original_url}
        descricao="O arquivo sai do storage e não há como desfazer."
        confirmarLabel="Sim, remover"
        processando={remover.isPending}
        erro={erroRemocao}
        onConfirm={confirmarRemocao}
        onCancel={cancelarRemocao}
      />
    </AdminShell>
  );
};
