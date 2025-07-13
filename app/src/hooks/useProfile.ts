import { useMutation } from "@tanstack/react-query";
import { useAuth } from "./useAuth";
import { updatePassword, updateProfile } from "../api/updateProfile";

export function useUpdateProfile() {
  const { token, refreshProfile } = useAuth();
  return useMutation({
    mutationFn: updateProfile(token || ""),
    onSuccess: () => {
      refreshProfile();
    },
    onError: (error) => {
      console.error("Erro ao atualizar perfil:", error);
    },
  });
}

export function useUpdatePassword() {
  const { token } = useAuth();
  return useMutation({
    mutationFn: updatePassword(token || ""),
    onSuccess: () => {
      console.log("Senha atualizada com sucesso");
    },
    onError: (error) => {
      console.error("Erro ao atualizar senha:", error);
    },
  });
}
