// hooks/useDownloadMutation.ts
import { useMutation } from "@tanstack/react-query";
import { useAuth } from "./useAuth";
import { createDownload } from "../api/createData";

export function useDownloadMutation() {
  const { token } = useAuth();
  return useMutation({
    mutationFn: createDownload(token || ""),
  });
}
