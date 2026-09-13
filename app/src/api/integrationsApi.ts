import type {
  ApiKey,
  ApiKeyCreated,
  DeliveriesResponse,
  IntegrationCreated,
  IntegrationDetail,
  IntegrationsResponse,
  RequestsResponse,
  TrafficResponse,
  IntegrationDownloadsResponse,
  Webhook,
} from "../interface/Integration";

const apiUrl = import.meta.env.VITE_API_URL;

// O mesmo tratamento de erro do resto do painel: a mensagem do servidor chega
// à tela, e uma falha de rede não vira "Failed to fetch".
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

function query(params: Record<string, string | number | undefined>): string {
  const search = new URLSearchParams();
  for (const [chave, valor] of Object.entries(params)) {
    if (valor !== undefined && valor !== "") search.set(chave, String(valor));
  }
  const texto = search.toString();
  return texto ? `?${texto}` : "";
}

const base = "/v1/admin/integrations";

// ---------------------------------------------------------------------------
// Integrações
// ---------------------------------------------------------------------------

export interface IntegrationFilters {
  page?: number;
  perPage?: number;
  search?: string;
  status?: string;
}

export function getIntegrations(token: string, filters: IntegrationFilters) {
  return () => request<IntegrationsResponse>(token, `${base}${query({ ...filters })}`);
}

export function getIntegration(token: string, id: string, windowHours: number) {
  return () =>
    request<IntegrationDetail>(
      token,
      `${base}/${id}${query({ window_hours: windowHours })}`
    );
}

export interface IntegrationPayload {
  name?: string;
  description?: string;
  callback_base_url?: string;
  daily_limit?: number;
  max_file_size_bytes?: number;
  rate_limit_per_minute?: number;
  max_concurrent_downloads?: number;
  allowed_ips?: string[];
  active?: boolean;
}

export function createIntegration(token: string) {
  return (payload: IntegrationPayload) =>
    request<IntegrationCreated>(token, base, {
      method: "POST",
      body: JSON.stringify(payload),
    });
}

export function updateIntegration(token: string) {
  return ({ id, ...payload }: IntegrationPayload & { id: string }) =>
    request<{ integration: IntegrationCreated["integration"] }>(token, `${base}/${id}`, {
      method: "PATCH",
      body: JSON.stringify(payload),
    });
}

export function deleteIntegration(token: string) {
  return (id: string) =>
    request<{ message: string }>(token, `${base}/${id}`, { method: "DELETE" });
}

// ---------------------------------------------------------------------------
// Chaves
// ---------------------------------------------------------------------------

export function getKeys(token: string, id: string) {
  return () => request<{ api_keys: ApiKey[] }>(token, `${base}/${id}/keys`);
}

export interface CreateKeyPayload {
  id: string;
  label?: string;
  expires_in_days?: number;
  // Com revoke_others, é a operação de REGERAR: a chave antiga deixa de valer
  // no mesmo instante.
  revoke_others?: boolean;
}

export function createKey(token: string) {
  return ({ id, ...payload }: CreateKeyPayload) =>
    request<ApiKeyCreated>(token, `${base}/${id}/keys`, {
      method: "POST",
      body: JSON.stringify(payload),
    });
}

export function revokeKey(token: string) {
  return ({ id, keyId }: { id: string; keyId: string }) =>
    request<{ api_key: ApiKey }>(token, `${base}/${id}/keys/${keyId}`, {
      method: "DELETE",
    });
}

// ---------------------------------------------------------------------------
// Webhooks
// ---------------------------------------------------------------------------

export function getWebhooks(token: string, id: string) {
  return () => request<{ webhooks: Webhook[] }>(token, `${base}/${id}/webhooks`);
}

export interface WebhookPayload {
  id: string;
  url?: string;
  events?: string[];
  include_progress?: boolean;
  active?: boolean;
  rotate_secret?: boolean;
}

export function createWebhook(token: string) {
  return ({ id, ...payload }: WebhookPayload) =>
    request<{ webhook: Webhook }>(token, `${base}/${id}/webhooks`, {
      method: "POST",
      body: JSON.stringify(payload),
    });
}

export function updateWebhook(token: string) {
  return ({ id, webhookId, ...payload }: WebhookPayload & { webhookId: string }) =>
    request<{ webhook: Webhook }>(token, `${base}/${id}/webhooks/${webhookId}`, {
      method: "PATCH",
      body: JSON.stringify(payload),
    });
}

export function deleteWebhook(token: string) {
  return ({ id, webhookId }: { id: string; webhookId: string }) =>
    request<{ message: string }>(token, `${base}/${id}/webhooks/${webhookId}`, {
      method: "DELETE",
    });
}

export function testWebhook(token: string) {
  return ({ id, webhookId }: { id: string; webhookId: string }) =>
    request<{ message: string }>(token, `${base}/${id}/webhooks/${webhookId}/test`, {
      method: "POST",
    });
}

// ---------------------------------------------------------------------------
// Entregas, auditoria e downloads
// ---------------------------------------------------------------------------

export interface DeliveryFilters {
  page?: number;
  perPage?: number;
  status?: string;
  event_type?: string;
}

export function getDeliveries(token: string, id: string, filters: DeliveryFilters) {
  return () =>
    request<DeliveriesResponse>(token, `${base}/${id}/deliveries${query({ ...filters })}`);
}

export function retryDelivery(token: string) {
  return ({ id, deliveryId }: { id: string; deliveryId: string }) =>
    request<{ message: string }>(
      token,
      `${base}/${id}/deliveries/${deliveryId}/retry`,
      { method: "POST" }
    );
}

export interface RequestFilters {
  page?: number;
  perPage?: number;
  ip?: string;
  status_class?: string;
  path?: string;
  error_code?: string;
}

export function getRequests(token: string, id: string, filters: RequestFilters) {
  return () =>
    request<RequestsResponse>(token, `${base}/${id}/requests${query({ ...filters })}`);
}

export function getTraffic(token: string, id: string, windowHours: number) {
  return () =>
    request<TrafficResponse>(
      token,
      `${base}/${id}/traffic${query({ window_hours: windowHours })}`
    );
}

export interface IntegrationDownloadFilters {
  page?: number;
  perPage?: number;
  status?: string;
  search?: string;
}

export function getIntegrationDownloads(
  token: string,
  id: string,
  filters: IntegrationDownloadFilters
) {
  return () =>
    request<IntegrationDownloadsResponse>(
      token,
      `${base}/${id}/downloads${query({ ...filters })}`
    );
}
