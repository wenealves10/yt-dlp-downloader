import React from "react";
import { useQuery } from "@tanstack/react-query";
import { Download } from "lucide-react";
import { useAuth } from "../../hooks/useAuth";
import { getDailyDownloads } from "../../api/getData";

export const DownloadCounter: React.FC = () => {
  const { token, user } = useAuth();

  const {
    data: downloadsDaily,
    isLoading,
    isError,
  } = useQuery({
    queryKey: ["downloadsDaily"],
    queryFn: () => getDailyDownloads(token || "")(),
    refetchOnWindowFocus: false,
    enabled: !!token,
  });

  if (!token || !user?.daily_limit) return null;

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 text-sm text-gray-400 animate-pulse">
        <Download className="w-4 h-4 text-gray-500" />
        <span>Carregando downloads...</span>
      </div>
    );
  }

  if (isError || !downloadsDaily) {
    return (
      <div className="flex items-center gap-2 text-sm text-red-400">
        <Download className="w-4 h-4" />
        <span>Erro ao carregar os downloads.</span>
      </div>
    );
  }

  const remaining = downloadsDaily.remaining;
  const current = remaining < 0 ? 0 : remaining;
  const limit = user.daily_limit;
  const isLow = current <= 1;

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
          {current}
        </span>{" "}
        de <span className="text-white font-medium">{limit}</span> downloads
        hoje.
      </span>
    </div>
  );
};
