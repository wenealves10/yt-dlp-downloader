import React, { useEffect, useState } from "react";
import {
  Youtube,
  Link,
  Clapperboard,
  Music,
  Download,
  Loader,
} from "lucide-react";
import { UserMenu } from "../user/UserMenu";
import { DownloadCard } from "./DownloadCard";
import { useAuth } from "../../hooks/useAuth";
import { useQuery } from "@tanstack/react-query";
import { getData } from "../../api/getData";
import { bucketHost } from "../../constants/config";
import { DownloadCounter } from "./DownloadCounter";
import {
  useDeleteDownloadMutation,
  useDownloadMutation,
} from "../../hooks/useDownloadMutation";
import { useDownloads } from "../../hooks/useDownload";
import type { Download as DownloadType } from "../../interface/Download";
const apiUrl = import.meta.env.VITE_API_URL;

interface Job {
  id: string;
  title?: string;
  status: "queue" | "processing" | "complete" | "expired" | "error";
  format: string;
  thumbnail?: string;
  downloadUrl?: string;
  durationSeconds?: number;
  completedAt?: number;
  expiresAt?: string | null;
  summary?: string;
  tweet?: string;
  isSummarizing?: boolean;
  isGeneratingTweet?: boolean;
}

function convertStatus(apiStatus: string): Job["status"] {
  switch (apiStatus) {
    case "PENDING":
      return "queue";
    case "PROCESSING":
      return "processing";
    case "COMPLETED":
      return "complete";
    case "EXPIRED":
      return "expired";
    case "FAILED":
      return "error";
    default:
      return "queue";
  }
}

export const DownloaderPage: React.FC = () => {
  const { user, logout, token } = useAuth();
  const [url, setUrl] = useState("");
  const [format, setFormat] = useState("video");
  const [jobs, setJobs] = useState<Job[]>([]);
  const [error, setError] = useState("");
  const [page, setPage] = useState(1);
  const [perPage] = useState(10);
  const { remaining, refetch } = useDownloads();
  const deleteDownload = useDeleteDownloadMutation();

  const downloadsQuery = useQuery({
    queryKey: ["downloads", page, perPage],
    queryFn: () => getData(token || "")(perPage, page),
    refetchOnWindowFocus: false,
    enabled: !!token,
  });

  const downloadMutation = useDownloadMutation();

  useEffect(() => {
    if (downloadsQuery.data?.downloads) {
      const mapped: Job[] = downloadsQuery.data.downloads.map((item) => ({
        id: item.id,
        title: item.title,
        status: convertStatus(item.status),
        durationSeconds: item.duration_seconds,
        format: item.format.toLowerCase(),
        thumbnail: item.thumbnail_url?.includes("http")
          ? ""
          : `${bucketHost}/${item.thumbnail_url}`,
        downloadUrl: `${bucketHost}/${item.file_url}`,
        completedAt: new Date(item.created_at).getTime(),
        expiresAt: item.expires_at,
      }));
      setJobs(mapped);
    }
  }, [downloadsQuery.data]);

  useEffect(() => {
    const eventSource = new EventSource(`${apiUrl}/v1/sse?token=${token}`);

    eventSource.onmessage = (event) => {
      console.log("Mensagem SSE:", event.data);
      const data: DownloadType = JSON.parse(event.data);
      setJobs((prev) => {
        const existingIndex = prev.findIndex((job) => job.id === data.id);
        if (existingIndex !== -1) {
          const updatedJobs = [...prev];
          updatedJobs[existingIndex] = {
            ...updatedJobs[existingIndex],
            ...data,
            status: convertStatus(data.status),
            thumbnail: data?.thumbnail_url
              ? `${bucketHost}/${data.thumbnail_url}`
              : updatedJobs[existingIndex].thumbnail,
            downloadUrl: data?.file_url ? `${bucketHost}/${data.file_url}` : "",
            expiresAt: data.expires_at || updatedJobs[existingIndex].expiresAt,
          };
          return updatedJobs;
        }
        return [...prev, { ...data, status: convertStatus(data.status) }];
      });
    };

    eventSource.onerror = (err) => {
      console.error("Erro SSE:", err);
      eventSource.close();
    };

    return () => {
      eventSource.close();
    };
  }, []);

  const handleDownload = (e: React.FormEvent) => {
    e.preventDefault();
    if (!url.includes("youtube.com/") && !url.includes("youtu.be/")) {
      setError("Link inválido.");
      return;
    }
    setError("");
    downloadMutation.mutate(
      { url, type: format as "music" | "video" },
      {
        onSuccess: () => {
          setUrl("");
          downloadsQuery.refetch();
          refetch();
        },
        onError: () => {
          setError("Erro ao processar o download.");
        },
      }
    );
    setUrl("");
  };

  const handleRemoveJob = (id: string) => {
    deleteDownload.mutate(id);
    setJobs((prev) => prev.filter((job) => job.id !== id));
  };

  return (
    <main className="bg-gray-900 text-white min-h-screen font-sans p-4 sm:p-6 lg:p-8">
      <div className="max-w-4xl mx-auto">
        <header className="mb-8 flex justify-between items-center">
          <div className="flex items-center gap-4">
            <div>
              <img src="/logo.svg" alt="AdVideo Logo" className="h-auto w-32" />
            </div>
          </div>
          <div className="flex items-center gap-4">
            <DownloadCounter />
            <UserMenu user={user} onLogout={logout} />
          </div>
        </header>

        <section className="bg-gray-800 p-6 rounded-xl shadow-2xl border border-gray-700">
          <form onSubmit={handleDownload}>
            <div className="relative mb-4">
              <Link className="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-gray-500" />
              <input
                type="text"
                value={url}
                onChange={(e) => {
                  setUrl(e.target.value);
                  setError("");
                }}
                disabled={remaining === 0}
                placeholder="Cole o link do YouTube aqui..."
                className={`w-full bg-gray-900 border border-gray-600 rounded-lg py-3 pl-10 pr-4 focus:ring-2 focus:ring-red-500 focus:border-red-500 transition-all placeholder-gray-500 
                    ${
                      remaining === 0 ? "opacity-50 cursor-not-allowed" : ""
                    }                  
                  `}
              />
            </div>
            {error && <p className="text-red-400 text-sm mb-4">{error}</p>}
            <div className="flex flex-col sm:flex-row items-center gap-4">
              <div className="flex-grow w-full sm:w-auto flex items-stretch gap-2 p-1 bg-gray-900 rounded-lg">
                <button
                  type="button"
                  onClick={() => setFormat("video")}
                  className={`flex-1 flex items-center justify-center gap-2 px-4 py-2 rounded-md transition-colors text-sm font-medium ${
                    format === "video"
                      ? "bg-red-600 text-white"
                      : "hover:bg-gray-700"
                  }`}
                >
                  <Clapperboard size={16} />
                  MP4
                </button>
                <button
                  type="button"
                  onClick={() => setFormat("music")}
                  className={`flex-1 flex items-center justify-center gap-2 px-4 py-2 rounded-md transition-colors text-sm font-medium ${
                    format === "music"
                      ? "bg-red-600 text-white"
                      : "hover:bg-gray-700"
                  }`}
                >
                  <Music size={16} />
                  MP3
                </button>
              </div>
              <button
                type="submit"
                disabled={downloadMutation.isPending || !url}
                className="w-full sm:w-auto flex-shrink-0 flex items-center justify-center gap-2 bg-red-600 hover:bg-red-700 text-white font-bold py-2.5 px-6 rounded-lg transition-all disabled:bg-red-800 disabled:cursor-not-allowed"
              >
                {downloadMutation.isPending ? (
                  <>
                    <Loader className="animate-spin" size={20} /> Processando...
                  </>
                ) : (
                  <>
                    <Download size={20} /> Baixar
                  </>
                )}
              </button>
            </div>
          </form>
          <p className="text-xs text-center text-gray-500 mt-4">
            Arquivos disponíveis por 24h.
          </p>
        </section>

        {jobs.length > 0 && (
          <section className="mt-8">
            <h2 className="text-2xl font-semibold mb-4 text-gray-300">
              Downloads Recentes
            </h2>
            <div className="space-y-4">
              {jobs.map((job) => (
                <DownloadCard
                  key={job.id}
                  job={job}
                  onRemove={handleRemoveJob}
                />
              ))}
            </div>
          </section>
        )}
      </div>
    </main>
  );
};
