import type {
  DailyDownloadsResponse,
  DownloadsResponse,
} from "../interface/Download";

const apiUrl = import.meta.env.VITE_API_URL;

type DownloadURLResponse = {
  url?: string;
  error?: string;
};

// The application API authenticates the user and returns a short-lived R2
// presigned URL. We never construct a bucket URL in the browser, so private
// objects cannot be requested merely by guessing their key.
export async function startFileDownload(
  downloadId: string,
  token: string
): Promise<void> {
  const response = await fetch(
    `${apiUrl}/v1/downloads/${encodeURIComponent(downloadId)}/download-url`,
    {
      method: "GET",
      headers: { Authorization: `Bearer ${token}` },
    }
  );
  const body = (await response.json().catch(() => ({}))) as DownloadURLResponse;

  if (!response.ok) {
    throw new Error(body.error || "Não foi possível preparar o download.");
  }
  if (!body.url) {
    throw new Error("A API não retornou uma URL de download válida.");
  }

  const link = document.createElement("a");
  link.href = body.url;
  link.style.display = "none";
  link.referrerPolicy = "no-referrer";
  document.body.appendChild(link);
  link.click();
  link.remove();
}

export function getData(token: string) {
  return async (perPage: number, page: number): Promise<DownloadsResponse> => {
    const res = await fetch(
      `${apiUrl}/v1/downloads?perPage=${perPage}&page=${page}`,
      {
        method: "GET",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (!res.ok) {
      throw new Error("Erro ao buscar downloads");
    }

    return res.json();
  };
}

export function getDailyDownloads(token: string) {
  return async (): Promise<DailyDownloadsResponse> => {
    const res = await fetch(`${apiUrl}/v1/downloads/daily`, {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${token}`,
      },
    });

    if (!res.ok) {
      throw new Error("Erro ao buscar downloads diários");
    }

    return res.json();
  };
}
