import type { CreatedDownload, ResolvedMedia } from "../interface/Media";
import { MediaError } from "../interface/Media";

const apiUrl = import.meta.env.VITE_API_URL;

async function request<T>(token: string, path: string, body: unknown): Promise<T> {
  let res: Response;
  try {
    res = await fetch(`${apiUrl}${path}`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify(body),
    });
  } catch {
    throw new MediaError(
      "Não foi possível falar com o servidor. Verifique a conexão.",
      "network"
    );
  }

  const dados = await res.json().catch(() => ({}));

  if (!res.ok) {
    throw new MediaError(
      dados?.error || dados?.message || "Não foi possível concluir a operação.",
      dados?.code || "unknown",
      dados?.detail,
      dados?.session
    );
  }
  return dados as T;
}

// resolve identifica a plataforma e traz os formatos. Não cria download nem
// consome o limite diário: é a consulta que antecede a escolha.
export function resolveMedia(token: string) {
  return (url: string) => request<ResolvedMedia>(token, "/v1/media/resolve", { url });
}

export function createMediaDownload(token: string) {
  return (payload: { url: string; format_id?: string; kind?: string }) =>
    request<CreatedDownload>(token, "/v1/media/downloads", payload);
}

export function cancelDownload(token: string) {
  return (id: string) =>
    request<{ id: string; status: string }>(
      token,
      `/v1/downloads/${encodeURIComponent(id)}/cancel`,
      {}
    );
}
