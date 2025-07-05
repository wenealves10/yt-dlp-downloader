export function deleteDownload(token: string) {
  return async (id: string) => {
    const res = await fetch(
      `${import.meta.env.VITE_API_URL}/v1/downloads/${id}`,
      {
        method: "DELETE",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
      }
    );
    if (!res.ok) {
      throw new Error("Erro ao deletar download");
    }
  };
}
