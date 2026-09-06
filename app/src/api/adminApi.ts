import type {
  AdminDownloadsResponse,
  AdminOverview,
  AdminUserDetail,
  AdminUsersResponse,
  UserMutationResponse,
} from "../interface/Admin";

const apiUrl = import.meta.env.VITE_API_URL;

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

// Um único objeto de filtros vira query string; chaves vazias são omitidas para
// não desligar filtros no backend por engano.
function query(params: Record<string, string | number | undefined>): string {
  const search = new URLSearchParams();
  for (const [chave, valor] of Object.entries(params)) {
    if (valor !== undefined && valor !== "") search.set(chave, String(valor));
  }
  const texto = search.toString();
  return texto ? `?${texto}` : "";
}

export interface OverviewFilters {
  range?: string;
  from?: string;
  to?: string;
}

export function getOverview(token: string, filters: OverviewFilters) {
  return () =>
    request<AdminOverview>(token, `/v1/admin/overview${query({ ...filters })}`);
}

export interface UserFilters {
  page?: number;
  perPage?: number;
  search?: string;
  plan?: string;
  role?: string;
  status?: string;
}

export function getUsers(token: string, filters: UserFilters) {
  return () =>
    request<AdminUsersResponse>(token, `/v1/admin/users${query({ ...filters })}`);
}

export function getUser(token: string, id: string) {
  return () => request<AdminUserDetail>(token, `/v1/admin/users/${id}`);
}

export interface CreateUserPayload {
  full_name: string;
  email: string;
  password?: string;
  plan?: string;
  role?: string;
  daily_limit?: number;
}

export function createUser(token: string) {
  return (payload: CreateUserPayload) =>
    request<UserMutationResponse>(token, "/v1/admin/users", {
      method: "POST",
      body: JSON.stringify(payload),
    });
}

export interface UpdateUserPayload {
  full_name?: string;
  email?: string;
  plan?: string;
  role?: string;
  daily_limit?: number;
  active?: boolean;
  is_verified?: boolean;
}

export function updateUser(token: string) {
  return ({ id, ...payload }: UpdateUserPayload & { id: string }) =>
    request<UserMutationResponse>(token, `/v1/admin/users/${id}`, {
      method: "PATCH",
      body: JSON.stringify(payload),
    });
}

export function resetPassword(token: string) {
  return ({ id, password }: { id: string; password?: string }) =>
    request<{ message: string; generated_password?: string }>(
      token,
      `/v1/admin/users/${id}/password`,
      { method: "POST", body: JSON.stringify({ password }) }
    );
}

export function deleteUser(token: string) {
  return (id: string) =>
    request<{ message: string }>(token, `/v1/admin/users/${id}`, {
      method: "DELETE",
    });
}

export interface DownloadFilters {
  page?: number;
  perPage?: number;
  search?: string;
  status?: string;
  user_id?: string;
}

export function getDownloads(token: string, filters: DownloadFilters) {
  return () =>
    request<AdminDownloadsResponse>(
      token,
      `/v1/admin/downloads${query({ ...filters })}`
    );
}

export function deleteDownload(token: string) {
  return (id: string) =>
    request<{ message: string }>(token, `/v1/admin/downloads/${id}`, {
      method: "DELETE",
    });
}
