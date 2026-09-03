import type {
  DailyDownloadsResponse,
  DownloadsResponse,
} from "../interface/Download";

const apiUrl = import.meta.env.VITE_API_URL;
const downloadFrameName = "download-proxy-frame";

function ensureDownloadFrame(): void {
  if (document.querySelector(`iframe[name="${downloadFrameName}"]`)) {
    return;
  }

  const frame = document.createElement("iframe");
  frame.name = downloadFrameName;
  frame.title = "Download";
  frame.hidden = true;
  document.body.appendChild(frame);
}

// A native form POST lets the browser consume the attachment response as soon
// as the API starts streaming it. It is targeted to a hidden frame so an API
// error never replaces the downloader page with a 404 response. Unlike a
// direct R2 link, it never asks the bucket domain to display or redirect the
// file.
export function startFileDownload(downloadId: string, token: string): void {
  ensureDownloadFrame();

  const form = document.createElement("form");
  form.method = "POST";
  form.action = `${apiUrl}/v1/downloads/${encodeURIComponent(downloadId)}/file`;
  form.target = downloadFrameName;
  form.style.display = "none";

  const accessToken = document.createElement("input");
  accessToken.type = "hidden";
  accessToken.name = "access_token";
  accessToken.value = token;
  form.appendChild(accessToken);

  document.body.appendChild(form);
  form.submit();
  form.remove();
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
