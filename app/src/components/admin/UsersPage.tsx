import React, { useState } from "react";
import { Link } from "react-router-dom";
import {
  ChevronLeft,
  ChevronRight,
  Loader,
  Pencil,
  Plus,
  Search,
  ShieldCheck,
  Trash2,
} from "lucide-react";
import { AdminShell } from "./AdminShell";
import { UserFormModal } from "./UserFormModal";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import { useAdminUsers, useDeleteUser } from "../../hooks/useAdmin";
import { formatBytes, formatDateTime, formatNumber } from "./format";
import type { AdminUser } from "../../interface/Admin";
import { useAuth } from "../../hooks/useAuth";

const CORES_PLANO: Record<string, string> = {
  free: "bg-gray-700 text-gray-300",
  premium: "bg-blue-600/20 text-blue-300 border border-blue-700/50",
  enterprise: "bg-violet-600/20 text-violet-300 border border-violet-700/50",
};

export const UsersPage: React.FC = () => {
  const { user: eu } = useAuth();
  const [page, setPage] = useState(1);
  const [busca, setBusca] = useState("");
  const [buscaAplicada, setBuscaAplicada] = useState("");
  const [plano, setPlano] = useState("");
  const [status, setStatus] = useState("");
  const [editando, setEditando] = useState<AdminUser | null>(null);
  const [criando, setCriando] = useState(false);
  // O usuário aguardando confirmação de remoção. Guardar o objeto inteiro, e
  // não só o id, deixa o diálogo mostrar de quem se trata.
  const [removendo, setRemovendo] = useState<AdminUser | null>(null);
  const [erroRemocao, setErroRemocao] = useState("");

  const filtros = {
    page,
    perPage: 20,
    search: buscaAplicada,
    plan: plano,
    status,
  };
  const { data, isLoading, isError, isFetching } = useAdminUsers(filtros);
  const remover = useDeleteUser();

  const aplicarBusca = (evento: React.FormEvent) => {
    evento.preventDefault();
    setPage(1);
    setBuscaAplicada(busca.trim());
  };

  const confirmarRemocao = () => {
    if (!removendo) return;

    setErroRemocao("");
    remover.mutate(removendo.id, {
      // O diálogo só fecha depois que o servidor confirma; fechar antes daria a
      // impressão de que deu certo mesmo quando a remoção é recusada.
      onSuccess: () => setRemovendo(null),
      onError: (falha) => setErroRemocao(falha.message),
    });
  };

  const cancelarRemocao = () => {
    setRemovendo(null);
    setErroRemocao("");
  };

  const trocarFiltro = (aplicar: () => void) => {
    setPage(1);
    aplicar();
  };

  return (
    <AdminShell
      titulo="Usuários"
      descricao="Planos, limites diários, acesso e histórico de cada conta."
      acoes={
        <button
          type="button"
          onClick={() => setCriando(true)}
          className="flex items-center gap-2 bg-red-600 hover:bg-red-700 text-white font-medium px-4 py-2 rounded-lg transition-colors"
        >
          <Plus size={16} />
          Novo usuário
        </button>
      }
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
            placeholder="Buscar por nome ou e-mail..."
            className="w-full bg-gray-800 border border-gray-700 rounded-lg py-2 pl-9 pr-3 text-sm placeholder-gray-500 focus:ring-2 focus:ring-red-500 focus:border-red-500 transition-all"
          />
        </form>

        <select
          value={plano}
          onChange={(evento) => trocarFiltro(() => setPlano(evento.target.value))}
          className="bg-gray-800 border border-gray-700 rounded-lg py-2 px-3 text-sm text-gray-200"
        >
          <option value="">Todos os planos</option>
          <option value="free">Free</option>
          <option value="premium">Premium</option>
          <option value="enterprise">Enterprise</option>
        </select>

        <select
          value={status}
          onChange={(evento) => trocarFiltro(() => setStatus(evento.target.value))}
          className="bg-gray-800 border border-gray-700 rounded-lg py-2 px-3 text-sm text-gray-200"
        >
          <option value="">Todos os status</option>
          <option value="active">Ativos</option>
          <option value="blocked">Bloqueados</option>
        </select>
      </div>

      {isLoading && (
        <p className="flex items-center gap-2 text-gray-400">
          <Loader className="animate-spin" size={18} /> Carregando usuários...
        </p>
      )}

      {isError && !isLoading && (
        <p className="text-red-400">Não foi possível carregar os usuários.</p>
      )}

      {data && (
        <>
          <div className="bg-gray-800 border border-gray-700 rounded-xl overflow-hidden">
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead className="bg-gray-800/80 border-b border-gray-700">
                  <tr className="text-left text-xs text-gray-400 uppercase tracking-wide">
                    <th className="px-4 py-3 font-medium">Usuário</th>
                    <th className="px-4 py-3 font-medium">Plano</th>
                    <th className="px-4 py-3 font-medium text-right">Limite/dia</th>
                    <th className="px-4 py-3 font-medium text-right">Downloads</th>
                    <th className="px-4 py-3 font-medium text-right">Storage</th>
                    <th className="px-4 py-3 font-medium">Último acesso</th>
                    <th className="px-4 py-3 font-medium text-right">Ações</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-700">
                  {data.users.map((usuario) => (
                    <tr
                      key={usuario.id}
                      className={`hover:bg-gray-700/30 transition-colors ${
                        usuario.active ? "" : "opacity-60"
                      }`}
                    >
                      <td className="px-4 py-3">
                        <div className="flex items-center gap-2">
                          <div>
                            <p className="text-gray-100 font-medium">
                              {usuario.full_name}
                            </p>
                            <p className="text-xs text-gray-500">{usuario.email}</p>
                          </div>
                          {usuario.role === "super_admin" && (
                            <span
                              className="text-red-400 shrink-0"
                              title="Super admin"
                            >
                              <ShieldCheck size={14} />
                            </span>
                          )}
                        </div>
                        {!usuario.active && (
                          <span className="inline-block mt-1 text-xs text-amber-400">
                            bloqueado
                          </span>
                        )}
                      </td>
                      <td className="px-4 py-3">
                        <span
                          className={`inline-block px-2 py-0.5 rounded-full text-xs font-medium ${
                            CORES_PLANO[usuario.plan] ?? CORES_PLANO.free
                          }`}
                        >
                          {usuario.plan}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-right text-gray-300 tabular-nums">
                        {usuario.role === "super_admin" ? (
                          <span className="text-gray-500">ilimitado</span>
                        ) : (
                          formatNumber(usuario.daily_limit)
                        )}
                      </td>
                      <td className="px-4 py-3 text-right text-gray-300 tabular-nums">
                        <Link
                          to={`/admin/downloads?user_id=${usuario.id}`}
                          className="hover:text-white hover:underline"
                        >
                          {formatNumber(usuario.downloads_total)}
                        </Link>
                      </td>
                      <td className="px-4 py-3 text-right text-gray-400 tabular-nums">
                        {formatBytes(usuario.storage_bytes)}
                      </td>
                      <td className="px-4 py-3 text-gray-400 text-xs">
                        {formatDateTime(usuario.last_login)}
                      </td>
                      <td className="px-4 py-3">
                        <div className="flex items-center justify-end gap-1">
                          <button
                            type="button"
                            onClick={() => setEditando(usuario)}
                            title="Editar"
                            className="p-2 rounded-md text-gray-400 hover:bg-gray-700 hover:text-white transition-colors"
                          >
                            <Pencil size={15} />
                          </button>
                          <button
                            type="button"
                            onClick={() => setRemovendo(usuario)}
                            disabled={usuario.id === eu?.id}
                            title={
                              usuario.id === eu?.id
                                ? "Você não pode remover a própria conta"
                                : "Remover"
                            }
                            className="p-2 rounded-md text-red-400 hover:bg-gray-700 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
                          >
                            <Trash2 size={15} />
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            {data.users.length === 0 && (
              <p className="py-12 text-center text-sm text-gray-500">
                Nenhum usuário encontrado com esses filtros.
              </p>
            )}
          </div>

          <div className="mt-4 flex items-center justify-between gap-4 text-sm">
            <p className="text-gray-500">
              {formatNumber(data.total)} usuário(s)
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

      {(criando || editando) && (
        <UserFormModal
          usuario={editando}
          aoFechar={() => {
            setCriando(false);
            setEditando(null);
          }}
        />
      )}

      <ConfirmDialog
        open={removendo !== null}
        titulo="Remover este usuário?"
        alvo={removendo?.email}
        descricao="A conta perde o acesso imediatamente. O histórico de downloads é preservado."
        confirmarLabel="Sim, remover"
        processando={remover.isPending}
        erro={erroRemocao}
        onConfirm={confirmarRemocao}
        onCancel={cancelarRemocao}
      />
    </AdminShell>
  );
};
