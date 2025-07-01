export function createDownload(token: string) {
  return async (payload: { url: string; type: "music" | "video" }) => {
    const res = await fetch(`${import.meta.env.VITE_API_URL}/v1/downloads`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify(payload),
    });

    if (!res.ok) {
      throw new Error("Erro ao criar download");
    }

    return res.json();
  };
}
