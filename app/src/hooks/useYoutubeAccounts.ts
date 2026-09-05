import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useAuth } from "./useAuth";
import {
  checkYoutubeAccount,
  closeYoutubeBrowser,
  createYoutubeAccount,
  deleteYoutubeAccount,
  getYoutubeAccounts,
  openYoutubeBrowser,
} from "../api/adminYoutube";

const accountsKey = ["admin", "youtube", "accounts"];

export function useYoutubeAccounts() {
  const { token } = useAuth();

  return useQuery({
    queryKey: accountsKey,
    queryFn: getYoutubeAccounts(token || ""),
    enabled: !!token,
    // O estado do navegador é runtime; um refetch periódico mantém o painel
    // coerente sem exigir recarregar a página.
    refetchInterval: 15_000,
  });
}

function useAccountMutation<TInput, TOutput>(
  mutationFn: (input: TInput) => Promise<TOutput>
) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: accountsKey });
    },
  });
}

export function useCreateYoutubeAccount() {
  const { token } = useAuth();
  return useAccountMutation(createYoutubeAccount(token || ""));
}

export function useDeleteYoutubeAccount() {
  const { token } = useAuth();
  return useAccountMutation(deleteYoutubeAccount(token || ""));
}

export function useCheckYoutubeAccount() {
  const { token } = useAuth();
  return useAccountMutation(checkYoutubeAccount(token || ""));
}

export function useOpenYoutubeBrowser() {
  const { token } = useAuth();
  return useAccountMutation(openYoutubeBrowser(token || ""));
}

export function useCloseYoutubeBrowser() {
  const { token } = useAuth();
  return useAccountMutation(closeYoutubeBrowser(token || ""));
}
