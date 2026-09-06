// Tipos do painel de Super Admin. Espelham as respostas de /v1/admin.

export type Plan = "free" | "premium" | "enterprise";
export type Role = "user" | "admin" | "super_admin";

export interface AdminUser {
  id: string;
  full_name: string;
  email: string;
  photo_url?: string;
  plan: Plan;
  role: Role;
  daily_limit: number;
  active: boolean;
  is_verified: boolean;
  last_login?: string;
  created_at: string;
  downloads_total: number;
  storage_bytes: number;
}

export interface AdminUsersResponse {
  users: AdminUser[];
  total: number;
  page: number;
  per_page: number;
  next_page: boolean;
  prev_page: boolean;
}

export interface AdminUserDetail {
  user: AdminUser;
  downloads_completed: number;
  downloads_failed: number;
  transferred_bytes: number;
}

// A senha só volta quando foi o painel que a gerou; senha escolhida pelo
// administrador não trafega de volta.
export interface UserMutationResponse {
  user: AdminUser;
  generated_password?: string;
}

export interface AdminDownload {
  id: string;
  user_id: string;
  user_name: string;
  user_email: string;
  title: string;
  original_url: string;
  format: string;
  status: string;
  file_size_bytes: number;
  duration_seconds: number;
  error_message?: string;
  expires_at?: string;
  created_at: string;
}

export interface AdminDownloadsResponse {
  downloads: AdminDownload[];
  total: number;
  page: number;
  per_page: number;
  next_page: boolean;
  prev_page: boolean;
}

export interface SeriesPoint {
  bucket: string;
  total: number;
  completed: number;
  failed: number;
  transferred_bytes: number;
}

export type RangeKey =
  | "today"
  | "yesterday"
  | "7d"
  | "15d"
  | "30d"
  | "90d"
  | "12m"
  | "custom";

export interface AdminOverview {
  period: { from: string; to: string; granularity: string };
  downloads: {
    total: number;
    completed: number;
    failed: number;
    processing: number;
    expired: number;
    active_users: number;
    transferred_bytes: number;
  };
  storage: {
    stored_bytes: number;
    stored_files: number;
    lifetime_bytes: number;
    freed_bytes: number;
    freed_files: number;
  };
  platform: {
    users_total: number;
    users_active: number;
    users_premium: number;
    users_new_30d: number;
    downloads_total: number;
  };
  series: SeriesPoint[];
  formats: { format: string; total: number; transferred_bytes: number }[] | null;
  top_users:
    | {
        id: string;
        full_name: string;
        email: string;
        plan: Plan;
        downloads_total: number;
        transferred_bytes: number;
      }[]
    | null;
}
