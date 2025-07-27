import React from "react";
import { Download } from "lucide-react";
import { useDownloads } from "../../hooks/useDownload";

export const DownloadCounter: React.FC = () => {
  const { remaining, limit, loading, error } = useDownloads();

  if (loading) {
    return (
      <div className="flex items-center gap-2 text-sm text-muted-foreground animate-pulse">
        <Download className="w-4 h-4" />
        <span>Carregando downloads...</span>
      </div>
    );
  }

  if (error || remaining === null || limit === null) {
    return (
      <div className="flex items-center gap-2 text-sm text-destructive">
        <Download className="w-4 h-4" />
        <span>Erro ao carregar os downloads.</span>
      </div>
    );
  }

  const isLow = remaining <= 1;

  return (
    <div className="flex flex-row sm:items-center gap-2 text-sm text-muted-foreground bg-background border border-border border-gray-600 rounded-xl p-3 sm:p-2 shadow-sm">
      <div className="flex items-center gap-1">
        <Download className="w-4 h-4 text-primary" />
        <span className="hidden sm:inline">Downloads disponíveis:</span>
      </div>
      <div className="sm:ml-2">
        <span
          className={`${
            isLow ? "text-destructive font-bold" : "text-primary font-medium"
          }`}
        >
          {remaining}
        </span>{" "}
        de <span className="text-primary font-medium">{limit}</span>
      </div>
    </div>
  );
};
