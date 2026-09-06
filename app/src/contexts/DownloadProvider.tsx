import React, { createContext, useState, useEffect } from "react";
import { useAuth } from "../hooks/useAuth";
import { getDailyDownloads } from "../api/getData";

export interface DownloadContextType {
  remaining: number | null;
  limit: number | null;
  // Quando verdadeiro, remaining e limit não significam nada: a conta não tem
  // teto diário.
  unlimited: boolean;
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
  const [unlimited, setUnlimited] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(false);

  const fetchDownloads = async () => {
    if (!token) return;
    setLoading(true);
    setError(false);
    try {
      const data = await getDailyDownloads(token)();
      setUnlimited(Boolean(data.unlimited));
      setRemaining(data.remaining < 0 ? 0 : data.remaining);
      // O limite vem da API, e não do perfil em cache: editar o limite pelo
      // painel precisa refletir aqui sem exigir novo login.
      setLimit(data.daily_limit);
    } catch (err) {
      console.error("Erro ao carregar downloads diários:", err);
      setError(true);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (token && user) {
      fetchDownloads();
    }
  }, [token, user]);

  return (
    <DownloadContext.Provider
      value={{ remaining, limit, unlimited, loading, error, refetch: fetchDownloads }}
    >
      {children}
    </DownloadContext.Provider>
  );
};
