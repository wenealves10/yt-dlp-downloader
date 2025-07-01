import React, { createContext, useState, useEffect } from "react";
import { useAuth } from "../hooks/useAuth";
import { getDailyDownloads } from "../api/getData";

export interface DownloadContextType {
  remaining: number | null;
  limit: number | null;
  loading: boolean;
  error: boolean;
  refetch: () => void;
}

export const DownloadContext = createContext<DownloadContextType | undefined>(
  undefined
);

export const DownloadProvider: React.FC<{ children: React.ReactNode }> = ({
  children,
}) => {
  const { token, user } = useAuth();
  const [remaining, setRemaining] = useState<number | null>(null);
  const [limit, setLimit] = useState<number | null>(user?.daily_limit || null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(false);

  const fetchDownloads = async () => {
    if (!token) return;
    setLoading(true);
    setError(false);
    try {
      const data = await getDailyDownloads(token)();
      setRemaining(data.remaining < 0 ? 0 : data.remaining);
      setLimit(user?.daily_limit || null);
    } catch (err) {
      console.error("Erro ao carregar downloads diários:", err);
      setError(true);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (token && user?.daily_limit) {
      fetchDownloads();
    }
  }, [token, user]);

  return (
    <DownloadContext.Provider
      value={{ remaining, limit, loading, error, refetch: fetchDownloads }}
    >
      {children}
    </DownloadContext.Provider>
  );
};
