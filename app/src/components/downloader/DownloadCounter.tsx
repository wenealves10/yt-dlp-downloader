import React from "react";
import { Download } from "lucide-react";
import { useDownloads } from "../../hooks/useDownload";

export const DownloadCounter: React.FC = () => {
  const { remaining, limit, loading, error } = useDownloads();

  if (loading) {
    return (
      <div className="flex items-center gap-2 text-sm text-gray-400 animate-pulse">
        <Download className="w-4 h-4 text-gray-500" />
        <span>Carregando downloads...</span>
      </div>
    );
  }

  if (error || remaining === null || limit === null) {
    return (
      <div className="flex items-center gap-2 text-sm text-red-400">
        <Download className="w-4 h-4" />
        <span>Erro ao carregar os downloads.</span>
      </div>
    );
  }

  const isLow = remaining <= 1;

  return (
    <div className="flex items-center gap-2 text-sm text-gray-400">
      <Download className="w-4 h-4 text-gray-500" />
      <span>
        Restam{" "}
        <span
          className={
            isLow ? "text-red-400 font-semibold" : "text-white font-medium"
          }
        >
          {remaining}
        </span>{" "}
        de <span className="text-white font-medium">{limit}</span> downloads
        hoje.
      </span>
    </div>
  );
};
