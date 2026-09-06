import React, { useEffect, useState } from "react";
import {
  AlertTriangle,
  Clapperboard,
  Download,
  Music,
  Trash2,
  X,
} from "lucide-react";
import { StatusBadge } from "../ui/StatusBadge";
import { ProgressBar } from "./ProgressBar";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import { formatBytes, formatDuration } from "./format";
import type { DownloadProgress } from "../../interface/Media";

interface Job {
  id: string;
  title?: string;
  status: "queue" | "processing" | "complete" | "expired" | "error" | "canceled";
  format: string;
  thumbnail?: string;
  completedAt?: number;
  durationSeconds?: number;
  expiresAt?: string | null;
  platform?: string;
  uploader?: string;
  quality?: string;
  sizeBytes?: number;
  progress?: DownloadProgress;
  errorMessage?: string;
  errorDetail?: string;
}

export function formatSeconds(seconds: number): string {
  const h = Math.floor(seconds / 3600)
    .toString()
    .padStart(2, "0");
  const m = Math.floor((seconds % 3600) / 60)
    .toString()
    .padStart(2, "0");
  const s = Math.floor(seconds % 60)
    .toString()
    .padStart(2, "0");

  return `${h}:${m}:${s}`;
}

interface DownloadCardProps {
  job: Job;
  // Só o super admin vê o motivo técnico; para o cliente final a falha é
  // sempre a mensagem genérica.
  mostrarDiagnostico?: boolean;
  onDownload: (id: string) => void;
  onRemove: (id: string) => void;
  onCancel?: (id: string) => void;
  isDownloading?: boolean;
}

export const DownloadCard: React.FC<DownloadCardProps> = ({
  job,
  onDownload,
  onRemove,
  onCancel,
  isDownloading = false,
  mostrarDiagnostico = false,
}) => {
  const [timeLeft, setTimeLeft] = useState("");
  const [confirmarRemocao, setConfirmarRemocao] = useState(false);

  const emAndamento = job.status === "processing" || job.status === "queue";

  useEffect(() => {
    if (job.status !== "complete" || !job.expiresAt) return;

    const calcular = () => {
      const expiraEm = new Date(job.expiresAt!).getTime();
      const restante = expiraEm - Date.now();

      if (restante <= 0) {
        setTimeLeft("Expirado");
        return;
      }

      const h = String(Math.floor((restante / (1000 * 60 * 60)) % 24)).padStart(2, "0");
      const m = String(Math.floor((restante / 1000 / 60) % 60)).padStart(2, "0");
      setTimeLeft(`${h}h ${m}m`);
    };

    calcular();
    // De minuto em minuto: um contador de segundos aqui só faria a lista
    // re-renderizar sem que ninguém esteja olhando o relógio.
    const id = setInterval(calcular, 60_000);
    return () => clearInterval(id);
  }, [job.status, job.expiresAt]);

  const ehAudio = job.format?.toLowerCase() === "mp3";

  return (
    <article className="bg-gray-800/50 border border-gray-700/60 rounded-xl p-4 transition-colors hover:border-gray-600">
      <div className="flex gap-4">
        <div className="relative shrink-0 self-start w-28 sm:w-36">
          {job.thumbnail ? (
            <img
              src={job.thumbnail}
              alt=""
              loading="lazy"
              className="w-full aspect-video object-cover rounded-lg bg-gray-800"
            />
          ) : (
            <div className="w-full aspect-video rounded-lg bg-gray-800 flex items-center justify-center">
              {ehAudio ? (
                <Music className="text-gray-600" size={22} />
              ) : (
                <Clapperboard className="text-gray-600" size={22} />
              )}
            </div>
          )}
          {job.durationSeconds ? (
            <span className="absolute bottom-1 right-1 px-1.5 py-0.5 rounded bg-black/80 text-white text-[11px] font-medium tabular-nums">
              {formatDuration(job.durationSeconds)}
            </span>
          ) : null}
        </div>

        <div className="min-w-0 flex-grow">
          {/* Uma linha: a lista precisa de altura previsível para rolar bem. */}
          <p
            className="truncate font-medium text-gray-100"
            title={job.title || undefined}
          >
            {job.title || "Carregando..."}
          </p>

          <div className="flex flex-wrap items-center gap-x-2.5 gap-y-1 mt-1.5 text-xs text-gray-400">
            <StatusBadge status={job.status} />
            {job.platform && <span className="text-gray-400">{job.platform}</span>}
            {job.uploader && (
              <span className="truncate max-w-[12rem]">{job.uploader}</span>
            )}
            {job.quality && <span>{job.quality}</span>}
            {job.sizeBytes ? <span>{formatBytes(job.sizeBytes)}</span> : null}
          </div>

          {emAndamento && (
            <div className="mt-3">
              <ProgressBar progresso={job.progress} />
            </div>
          )}

          {job.status === "error" && job.errorMessage && (
            <div className="mt-2">
              <p className="flex items-start gap-1.5 text-sm text-red-400">
                <AlertTriangle size={14} className="mt-0.5 shrink-0" />
                {job.errorMessage}
              </p>
              {mostrarDiagnostico && job.errorDetail && (
                <p className="mt-1 pl-5 text-xs font-mono text-red-400/60 break-words">
                  {job.errorDetail}
                </p>
              )}
            </div>
          )}

          {job.status === "complete" && timeLeft && (
            <p className="mt-2 text-xs text-gray-400">
              Disponível por mais {timeLeft}
            </p>
          )}
        </div>

        <div className="shrink-0 flex flex-col items-end justify-between gap-2">
          {job.status === "complete" && (
            <button
              type="button"
              onClick={() => onDownload(job.id)}
              disabled={isDownloading}
              className="flex items-center gap-2 bg-blue-600 hover:bg-blue-700 disabled:bg-blue-800 disabled:cursor-wait text-white font-medium text-sm py-2 px-4 rounded-lg transition-colors"
            >
              <Download size={15} />
              <span className="hidden sm:inline">
                {isDownloading ? "Preparando..." : "Baixar"}
              </span>
            </button>
          )}

          {emAndamento && onCancel && (
            <button
              type="button"
              onClick={() => onCancel(job.id)}
              className="flex items-center gap-1.5 border border-gray-600 text-gray-300 hover:bg-gray-700 hover:text-white text-sm py-2 px-3 rounded-lg transition-colors"
            >
              <X size={14} />
              <span className="hidden sm:inline">Cancelar</span>
            </button>
          )}

          {!emAndamento && (
            <button
              type="button"
              onClick={() => setConfirmarRemocao(true)}
              className="p-2 rounded-md text-gray-500 hover:text-red-400 hover:bg-gray-700/60 transition-colors"
              aria-label="Remover do histórico"
              title="Remover do histórico"
            >
              <Trash2 size={15} />
            </button>
          )}
        </div>
      </div>

      <ConfirmDialog
        open={confirmarRemocao}
        titulo="Remover este download?"
        alvo={job.title}
        descricao="O arquivo sai do storage e não há como desfazer."
        confirmarLabel="Sim, remover"
        onConfirm={() => {
          onRemove(job.id);
          setConfirmarRemocao(false);
        }}
        onCancel={() => setConfirmarRemocao(false)}
      />
    </article>
  );
};
