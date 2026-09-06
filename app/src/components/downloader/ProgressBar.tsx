import React from "react";
import { Loader } from "lucide-react";
import type { DownloadProgress } from "../../interface/Media";
import { formatBytes, formatETA, formatSpeed } from "./format";

interface Props {
  progresso?: DownloadProgress;
}

// ProgressBar mostra o andamento do download. Quando a plataforma não informa
// o tamanho total, a barra vira indeterminada em vez de mentir um percentual.
export const ProgressBar: React.FC<Props> = ({ progresso }) => {
  const indeterminado = !progresso || progresso.total_bytes <= 0;
  const percentual = Math.min(100, Math.max(0, progresso?.percent ?? 0));

  if (progresso?.postprocess) {
    return (
      <div className="w-full">
        <div className="h-1.5 w-full bg-gray-700 rounded-full overflow-hidden">
          <div className="h-full w-full bg-blue-500 animate-pulse" />
        </div>
        <p className="flex items-center gap-1.5 mt-1.5 text-xs text-gray-300">
          <Loader className="animate-spin" size={12} />
          Finalizando o arquivo...
        </p>
      </div>
    );
  }

  return (
    <div className="w-full">
      <div className="h-1.5 w-full bg-gray-700 rounded-full overflow-hidden">
        {indeterminado ? (
          <div className="h-full w-1/3 bg-blue-500 animate-pulse" />
        ) : (
          <div
            className="h-full bg-blue-500 rounded-full transition-[width] duration-500 ease-out"
            style={{ width: `${percentual}%` }}
          />
        )}
      </div>

      <div className="flex flex-wrap items-center gap-x-3 gap-y-0.5 mt-1.5 text-xs text-gray-400 tabular-nums">
        {!indeterminado && (
          <span className="text-gray-200 font-medium">
            {percentual.toFixed(0)}%
          </span>
        )}
        {progresso && progresso.downloaded_bytes > 0 && (
          <span>
            {formatBytes(progresso.downloaded_bytes)}
            {progresso.total_bytes > 0 && ` / ${formatBytes(progresso.total_bytes)}`}
          </span>
        )}
        {progresso && progresso.speed_bps > 0 && (
          <span>{formatSpeed(progresso.speed_bps)}</span>
        )}
        {progresso && progresso.eta_seconds > 0 && (
          <span>ETA {formatETA(progresso.eta_seconds)}</span>
        )}
      </div>
    </div>
  );
};
