// hooks/useDownloadMutation.ts
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useAuth } from "./useAuth";
import { createDownload } from "../api/createData";
import { deleteDownload } from "../api/deleteData";
import { useDownloads } from "./useDownload";

export function useDownloadMutation() {
  const { token } = useAuth();
  return useMutation({
    mutationFn: createDownload(token || ""),
  });
}

export function useDeleteDownloadMutation() {
  const queryClient = useQueryClient();
  const { token } = useAuth();
  const { refetch } = useDownloads();

  return useMutation({
    mutationFn: deleteDownload(token || ""),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["downloads"] });
      refetch();
    },
    onError: (error) => {
      console.error("Erro ao deletar:", error);
    },
  });
}
