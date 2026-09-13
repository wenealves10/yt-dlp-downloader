// Tipos da seção de integrações: as contas de SISTEMA que consomem a
// plataforma por API.

export interface Integration {
  id: string;
  name: string;
  description?: string;
  callback_base_url?: string;
  daily_limit: number;
  max_file_size_bytes: number;
  rate_limit_per_minute: number;
  max_concurrent_downloads: number;
  allowed_ips: string[];
  active: boolean;
  service_email?: string;
  created_at?: string;
  updated_at?: string;

  // Números que vêm na listagem, na mesma consulta.
  downloads_total: number;
  downloads_today: number;
  downloads_active: number;
  storage_bytes: number;
  active_keys: number;
  last_used_at?: string;
}

export interface IntegrationsResponse {
  integrations: Integration[];
  total: number;
  page: number;
  per_page: number;
  next_page: boolean;
  prev_page: boolean;
}

export interface IntegrationDetail {
  integration: Integration;
  downloads: {
    total: number;
    today: number;
    active: number;
    completed: number;
    failed: number;
    canceled: number;
    storage_bytes: number;
    transferred_bytes: number;
  };
  quota: {
    daily_limit: number;
    daily_used: number;
    daily_remaining: number;
    resets_at: string;
  };
  requests: {
    window_hours: number;
    total: number;
    ok: number;
    client_errors: number;
    server_errors: number;
    throttled: number;
    unique_ips: number;
    avg_duration_ms: number;
    max_duration_ms: number;
  };
  deliveries: {
    window_hours: number;
    total: number;
    delivered: number;
    failed: number;
    pending: number;
    avg_duration_ms: number;
  };
}

export interface ApiKey {
  id: string;
  label: string;
  prefix: string;
  // O valor mascarado é tudo que a tela tem: o banco guarda só o hash.
  masked: string;
  revoked: boolean;
  revoked_at?: string;
  revoked_reason?: string;
  expires_at?: string;
  last_used_at?: string;
  last_used_ip?: string;
  created_at?: string;
}

// A resposta que traz a chave em claro. Acontece uma única vez, na criação:
// depois disso não existe forma de recuperá-la.
export interface ApiKeyCreated {
  api_key: ApiKey;
  api_key_plaintext: string;
  revoked_previous_keys?: boolean;
}

export interface IntegrationCreated {
  integration: Integration;
  api_key?: ApiKey;
  api_key_plaintext?: string;
  warning?: string;
}

export interface Webhook {
  id: string;
  url: string;
  events: string[];
  include_progress: boolean;
  active: boolean;
  last_delivery_at?: string;
  last_status_code?: number;
  last_error?: string;
  consecutive_failures: number;
  disabled_reason?: string;
  created_at?: string;
  // Só o painel recebe o segredo: é ele que o cliente usa para verificar a
  // assinatura do nosso POST.
  secret?: string;
}

export interface Delivery {
  id: string;
  webhook_id: string;
  webhook_url: string;
  event_type: string;
  download_id?: string;
  status: "PENDING" | "DELIVERED" | "FAILED";
  attempts: number;
  last_status_code?: number;
  last_error?: string;
  duration_ms: number;
  payload: unknown;
  created_at?: string;
  delivered_at?: string;
}

export interface DeliveriesResponse {
  deliveries: Delivery[];
  total: number;
  page: number;
  per_page: number;
  next_page: boolean;
  prev_page: boolean;
}

export interface IntegrationRequest {
  id: number;
  method: string;
  path: string;
  status_code: number;
  ip: string;
  user_agent: string;
  duration_ms: number;
  error_code?: string;
  api_key_id?: string;
  download_id?: string;
  created_at?: string;
}

export interface RequestsResponse {
  requests: IntegrationRequest[];
  total: number;
  page: number;
  per_page: number;
  next_page: boolean;
  prev_page: boolean;
}

export interface TrafficResponse {
  window_hours: number;
  top_ips: {
    ip: string;
    requests: number;
    errors: number;
    last_seen?: string;
    // Diz se aquele IP passaria pela lista atual: é o que transforma a tabela
    // em ação em vez de só informação.
    allowed: boolean;
  }[];
  top_errors: {
    error_code: string;
    status_code: number;
    occurrences: number;
    last_seen?: string;
  }[];
}

// O download como a rota do painel devolve. Difere de AdminDownload: aqui já se
// sabe de quem é (a integração da tela), então não vem nome nem e-mail de dono,
// e vem o detalhe técnico da falha, que é para o operador.
export interface IntegrationDownload {
  id: string;
  title: string;
  original_url: string;
  format: string;
  status: string;
  thumbnail_url?: string;
  expires_at?: string;
  duration_seconds?: number;
  error_message?: string;
  error_detail?: string;
  created_at: string;
  platform?: string;
  platform_label?: string;
  provider?: string;
  quality_label?: string;
  uploader?: string;
  file_size_bytes?: number;
}

export interface IntegrationDownloadsResponse {
  downloads: IntegrationDownload[];
  total: number;
  page: number;
  per_page: number;
  next_page: boolean;
  prev_page: boolean;
}

// Os eventos que um webhook pode assinar. A lista espelha o vocabulário aceito
// pela API; um valor fora dela é recusado no cadastro.
export const EVENTOS_WEBHOOK = [
  "download.queued",
  "download.started",
  "download.completed",
  "download.failed",
  "download.canceled",
  "download.expired",
  "download.retrying",
] as const;
