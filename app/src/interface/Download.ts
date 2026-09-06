import type { DownloadProgress } from "./Media";

export interface Download {
  id: string;
  title: string;
  original_url: string;
  format: "MP3" | "M4A" | "MP4" | "WEBM" | "BEST" | "FLAC";
  thumbnail_url: string;
  file_url: string;
  expires_at: string; // ISO date string
  duration_seconds: number;
  status:
    | "PENDING"
    | "PROCESSING"
    | "COMPLETED"
    | "FAILED"
    | "CANCELED"
    | "EXPIRED"
    | "RETRYING";
  created_at: string; // ISO date string
  platform?: string;
  platform_label?: string;
  provider?: string;
  quality_label?: string;
  uploader?: string;
  file_size_bytes?: number;
  error_message?: string;
  // Motivo técnico da falha. A API só o envia ao super admin; o cliente final
  // recebe apenas error_message, que é genérica.
  error_detail?: string;
  // Só chega pelo evento SSE; o histórico devolve o último estado gravado.
  progress?: DownloadProgress;
}

export interface DownloadsResponse {
  downloads: Download[];
  next_page: boolean;
  prev_page: boolean;
  page: number;
  per_page: number;
  total: number;
}

export interface DailyDownloadsResponse {
  daily_downloads: number;
  daily_limit: number;
  remaining: number;
  // Super admin não tem teto: sem esta flag, remaining=0 travaria o campo de
  // URL logo no primeiro download.
  unlimited: boolean;
}
