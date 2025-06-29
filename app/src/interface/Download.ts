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
}

export interface DownloadsResponse {
  downloads: Download[];
  next_page: boolean;
  prev_page: boolean;
  page: number;
  per_page: number;
  total: number;
}
