// Estados de sessão devolvidos pela API. Espelham o enum
// core.youtube_account_status do banco.
export type YoutubeAccountStatus =
  | "NOT_CONFIGURED"
  | "AWAITING_LOGIN"
  | "AUTHENTICATED"
  | "REQUIRES_AUTH"
  | "DISABLED"
  | "ERROR";

// Estado runtime do navegador daquela conta.
export type BrowserState = "STOPPED" | "STARTING" | "RUNNING" | "ERROR";

// Nenhum campo aqui carrega cookie, token ou senha: a API nunca expõe material
// de sessão ao frontend.
export interface YoutubeAccount {
  id: string;
  label: string;
  email?: string;
  status: YoutubeAccountStatus;
  browser_state: BrowserState;
  active: boolean;
  priority: number;
  viewers: number;
  last_error?: string;
  last_checked_at?: string;
  last_authenticated_at?: string;
  last_used_at?: string;
  // Marca a conta que o próximo download vai escolher no rodízio.
  next_in_rotation: boolean;
  browser_started_at?: string;
  browser_activity_at?: string;
  created_at: string;
}

export interface YoutubeAccountsResponse {
  accounts: YoutubeAccount[];
  browser_available: boolean;
  // Quantas contas o rodízio tem disponíveis neste momento.
  authenticated_count: number;
}

// Credenciais efêmeras para abrir o canal do navegador remoto. O ticket é de
// uso único e expira em segundos.
export interface BrowserSession {
  account: YoutubeAccount;
  ticket: string;
  ws_path: string;
  vnc_password?: string;
  width: number;
  height: number;
  ticket_expires_in_seconds: number;
}
