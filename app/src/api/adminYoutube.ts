import type {
  BrowserSession,
  YoutubeAccount,
  YoutubeAccountsResponse,
} from "../interface/YoutubeAccount";

const apiUrl = import.meta.env.VITE_API_URL;
const basePath = "/v1/admin/youtube/accounts";

async function request<T>(
  token: string,
  path: string,
  init?: RequestInit
): Promise<T> {
  let res: Response;
  try {
    res = await fetch(`${apiUrl}${path}`, {
      ...init,
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${token}`,
        ...(init?.headers || {}),
      },
    });
  } catch {
    // fetch só rejeita quando a requisição nem chegou ao servidor. O erro
    // nativo é "Failed to fetch", que não diz nada a quem está usando o painel.
    throw new Error(
      "Não foi possível falar com o servidor. Verifique a conexão e tente de novo."
    );
  }

  if (!res.ok) {
    let message = "Não foi possível concluir a operação.";
    try {
      const body = await res.json();
      if (body?.error) message = body.error;
    } catch {
      // Resposta sem corpo JSON: mantém a mensagem padrão.
    }
    throw new Error(message);
  }

  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

export function getYoutubeAccounts(token: string) {
  return () => request<YoutubeAccountsResponse>(token, basePath);
}

export function createYoutubeAccount(token: string) {
  return (payload: { label: string; email?: string }) =>
    request<YoutubeAccount>(token, basePath, {
      method: "POST",
      body: JSON.stringify(payload),
    });
}

export function deleteYoutubeAccount(token: string) {
  return (id: string) =>
    request<{ message: string }>(token, `${basePath}/${id}`, {
      method: "DELETE",
    });
}

export function checkYoutubeAccount(token: string) {
  return (id: string) =>
    request<YoutubeAccount>(token, `${basePath}/${id}/check`, {
      method: "POST",
    });
}

// Abre o navegador remoto (ou reaproveita o que já estiver aberto) e devolve o
// ticket de uso único para o WebSocket.
export function openYoutubeBrowser(token: string) {
  return (id: string) =>
    request<BrowserSession>(token, `${basePath}/${id}/browser`, {
      method: "POST",
    });
}

// Renova o ticket sem reabrir o navegador, para reconectar a tela.
export function refreshBrowserTicket(token: string) {
  return (id: string) =>
    request<BrowserSession>(token, `${basePath}/${id}/browser/ticket`, {
      method: "POST",
    });
}

// Fecha o navegador; a sessão continua salva no perfil persistente.
export function closeYoutubeBrowser(token: string) {
  return (id: string) =>
    request<YoutubeAccount>(token, `${basePath}/${id}/browser`, {
      method: "DELETE",
    });
}

// Converte o caminho devolvido pela API em URL de WebSocket, respeitando o
// esquema da própria API (ws em http, wss em https).
export function buildBrowserSocketURL(wsPath: string, ticket: string): string {
  const base = new URL(apiUrl, window.location.origin);
  base.protocol = base.protocol === "https:" ? "wss:" : "ws:";
  base.pathname = wsPath;
  base.search = `?ticket=${encodeURIComponent(ticket)}`;
  return base.toString();
}
