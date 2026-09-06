// Tipos do downloader multiplataforma. Espelham /v1/media.

export interface MediaFormat {
  id: string;
  label: string;
  kind: "video" | "audio";
  ext: string;
  height?: number;
  fps?: number;
  size_bytes?: number;
  size_approximate?: boolean;
}

export interface ResolvedMedia {
  url: string;
  platform: string;
  platform_label: string;
  provider: string;
  content_id: string;
  title: string;
  description?: string;
  thumbnail: string;
  duration: number;
  // O servidor já validou uploader_url como http(s); ainda assim a tela só a
  // usa em href, nunca em innerHTML.
  uploader?: string;
  uploader_id?: string;
  uploader_url?: string;
  follower_count?: number;
  view_count?: number;
  like_count?: number;
  upload_date?: string;
  webpage_url?: string;
  formats: MediaFormat[];
}

export interface CreatedDownload {
  id: string;
  status: string;
  title: string;
  platform: string;
  platform_label: string;
  quality_label: string;
  thumbnail_url: string;
  created_at: string;
}

// Progresso em tempo real, entregue por SSE. Todos os campos são de melhor
// esforço: nem toda plataforma informa tamanho total ou ETA.
export interface DownloadProgress {
  percent: number;
  downloaded_bytes: number;
  total_bytes: number;
  speed_bps: number;
  eta_seconds: number;
  postprocess: boolean;
}

// O código é estável; a mensagem pode mudar sem quebrar o tratamento na tela.
// O campo é declarado e atribuído separadamente porque o projeto compila com
// erasableSyntaxOnly, que proíbe parâmetros de propriedade no construtor.
export class MediaError extends Error {
  readonly code: string;

  constructor(message: string, code: string) {
    super(message);
    this.name = "MediaError";
    this.code = code;
  }
}
