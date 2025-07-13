export type UpdateProfileInput = {
  full_name?: string;
  photo?: File | null;
};

export function updateProfile(token: string) {
  return async (payload: UpdateProfileInput) => {
    const formData = new FormData();
    if (payload.full_name) formData.append("full_name", payload.full_name);
    if (payload.photo) formData.append("photo", payload.photo);

    const res = await fetch(`${import.meta.env.VITE_API_URL}/v1/profile`, {
      method: "PATCH",
      headers: {
        Authorization: `Bearer ${token}`,
      },
      body: formData,
    });

    if (!res.ok) {
      throw new Error("Erro ao atualizar perfil");
    }

    return res.json();
  };
}

export interface UpdatePasswordPayload {
  current_password: string;
  new_password: string;
}

export function updatePassword(token: string) {
  return async (payload: UpdatePasswordPayload) => {
    const res = await fetch(
      `${import.meta.env.VITE_API_URL}/v1/profile/change-password`,
      {
        method: "PUT",
        headers: {
          Authorization: `Bearer ${token}`,
          "Content-Type": "application/json",
        },
        body: JSON.stringify(payload),
      }
    );
    if (!res.ok) {
      throw new Error("Erro ao atualizar senha");
    }
    return res.json();
  };
}
