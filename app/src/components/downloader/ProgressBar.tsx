import React from "react";
import type { DownloadProgress } from "../../interface/Media";
import { formatBytes } from "./format";

interface Props {
  progresso?: DownloadProgress;
}

/**
 * ProgressBar mostra o andamento do download.
 *
 * Deliberadamente enxuta: só o percentual e o quanto já veio. Velocidade e ETA
 * são medidos por FAIXA — com vídeo e áudio baixados em sequência, os dois
 * reiniciavam no meio do caminho e contradiziam a barra, que é justamente o que
 * fazia o andamento parecer bugado.
 *
 * Quando a plataforma não informa o tamanho total, a barra vira indeterminada
 * em vez de mentir um percentual.
 */
export const ProgressBar: React.FC<Props> = ({ progresso }) => {
  const indeterminado = !progresso || progresso.total_bytes <= 0;
  const percentual = Math.min(100, Math.max(0, progresso?.percent ?? 0));
  const finalizando = !!progresso?.postprocess;

  return (
    <div className="w-full">
      <div className="h-1.5 w-full bg-gray-700 rounded-full overflow-hidden">
        {indeterminado ? (
          <div className="h-full w-1/3 bg-blue-500 animate-pulse" />
        ) : (
          // Na finalização a barra MANTÉM a largura que alcançou. Trocá-la por
          // uma animação de largura cheia lia como um recomeço.
          <div
            className={`h-full bg-blue-500 rounded-full transition-[width] duration-500 ease-out ${
              finalizando ? "animate-pulse" : ""
            }`}
            style={{ width: `${percentual}%` }}
          />
        )}
      </div>

      <div className="flex items-center gap-x-3 mt-1.5 text-xs text-gray-400 tabular-nums">
        {!indeterminado && (
          <span className="text-gray-200 font-medium">
            {percentual.toFixed(0)}%
          </span>
        )}
        {progresso && progresso.downloaded_bytes > 0 && (
          <span>
            {formatBytes(progresso.downloaded_bytes)}
            {progresso.total_bytes > 0 &&
              ` de ${formatBytes(progresso.total_bytes)}`}
          </span>
        )}
        {finalizando && <span className="text-gray-300">Finalizando...</span>}
      </div>
    </div>
  );
};
