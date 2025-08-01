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
      // Tenta ler o JSON da resposta de erro
      let errorData;
      try {
        errorData = await res.json();
      } catch {
        throw new Error("Erro ao criar download");
      }

      // Trata erro específico de tamanho excedido
      if (res.status === 400 && errorData?.code === "limit_exceeded") {
        const limitBytes = errorData.limit;
        const sizeBytes = errorData.size;

        const formatSize = (bytes: number): string => {
          if (bytes >= 1024 * 1024 * 1024) {
            return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
          } else {
            return `${(bytes / (1024 * 1024)).toFixed(0)} MB`;
          }
        };

        const limitFormatted = formatSize(limitBytes);
        const sizeFormatted = formatSize(sizeBytes);

        // Mensagem final pro usuário
        const message = `🚫 O vídeo ultrapassa o limite do seu plano atual.\n📦 Tamanho do vídeo: ${sizeFormatted}\n📉 Limite do plano: ${limitFormatted}`;

        throw new Error(message);
      }

      // Outros erros 400 genéricos
      throw new Error(errorData?.message || "Erro ao criar download");
    }

    return res.json();
  };
}
