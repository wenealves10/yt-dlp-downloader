import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useAuth } from "./useAuth";
import {
  createUser,
  deleteDownload,
  deleteUser,
  getDownloads,
  getOverview,
  getUser,
  getUsers,
  resetPassword,
  updateUser,
  type DownloadFilters,
  type OverviewFilters,
  type UserFilters,
} from "../api/adminApi";

const raiz = ["admin"];

export function useOverview(filters: OverviewFilters) {
  const { token } = useAuth();
  return useQuery({
    queryKey: [...raiz, "overview", filters],
    queryFn: getOverview(token || "", filters),
    enabled: !!token,
    // O painel fica aberto em uma aba enquanto downloads acontecem; sem isso os
    // números congelam na hora em que a tela foi carregada.
    refetchInterval: 60_000,
  });
}

export function useAdminUsers(filters: UserFilters) {
  const { token } = useAuth();
  return useQuery({
    queryKey: [...raiz, "users", filters],
    queryFn: getUsers(token || "", filters),
    enabled: !!token,
    // Mantém a página anterior na tela durante a troca de página, em vez de
    // piscar uma tabela vazia.
    placeholderData: (anterior) => anterior,
  });
}

export function useAdminUser(id: string | undefined) {
  const { token } = useAuth();
  return useQuery({
    queryKey: [...raiz, "user", id],
    queryFn: getUser(token || "", id || ""),
    enabled: !!token && !!id,
  });
}

// invalidarAdmin derruba o cache das telas que compartilham os mesmos números:
// mexer num usuário muda a listagem e também os totais do dashboard.
function useInvalidarAdmin() {
  const queryClient = useQueryClient();
  return () => queryClient.invalidateQueries({ queryKey: raiz });
}

export function useCreateUser() {
  const { token } = useAuth();
  const invalidar = useInvalidarAdmin();
  return useMutation({ mutationFn: createUser(token || ""), onSuccess: invalidar });
}

export function useUpdateUser() {
  const { token } = useAuth();
  const invalidar = useInvalidarAdmin();
  return useMutation({ mutationFn: updateUser(token || ""), onSuccess: invalidar });
}

export function useResetPassword() {
  const { token } = useAuth();
  return useMutation({ mutationFn: resetPassword(token || "") });
}

export function useDeleteUser() {
  const { token } = useAuth();
  const invalidar = useInvalidarAdmin();
  return useMutation({ mutationFn: deleteUser(token || ""), onSuccess: invalidar });
}

export function useAdminDownloads(filters: DownloadFilters) {
  const { token } = useAuth();
  return useQuery({
    queryKey: [...raiz, "downloads", filters],
    queryFn: getDownloads(token || "", filters),
    enabled: !!token,
    placeholderData: (anterior) => anterior,
  });
}

export function useDeleteDownload() {
  const { token } = useAuth();
  const invalidar = useInvalidarAdmin();
  return useMutation({ mutationFn: deleteDownload(token || ""), onSuccess: invalidar });
}
