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

interface Job {
  id: string;
  title?: string;
  status: "queue" | "processing" | "complete" | "error";
  format: string;
  thumbnail?: string;
  downloadUrl?: string;
  durationSeconds?: number;
  completedAt?: number;
  summary?: string;
  tweet?: string;
  isSummarizing?: boolean;
  isGeneratingTweet?: boolean;
}

function convertStatus(apiStatus: string): Job["status"] {
  switch (apiStatus) {
    case "PENDING":
    case "PROCESSING":
      return "processing";
    case "COMPLETED":
      return "complete";
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
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState("");
  const [page, setPage] = useState(1);
  const [perPage] = useState(10);

  const query = useQuery({
    queryKey: ["downloads", page, perPage],
    queryFn: () => getData(token || "")(perPage, page),
    refetchOnWindowFocus: false,
    enabled: !!token,
  });

  useEffect(() => {
    if (query.data?.downloads) {
      const mapped: Job[] = query.data.downloads.map((item) => ({
        id: item.id,
        title: item.title,
        status: convertStatus(item.status),
        durationSeconds: item.duration_seconds,
        format: item.format.toLowerCase(),
        thumbnail: `${bucketHost}/${item.thumbnail_url}`,
        downloadUrl: `${bucketHost}/${item.file_url}`,
        completedAt: new Date(item.created_at).getTime(),
      }));

      setJobs(mapped);
    }
  }, [query.data]);

  const handleDownload = (e: React.FormEvent) => {
    e.preventDefault();
    if (!url.includes("youtube.com/") && !url.includes("youtu.be/")) {
      setError("Link inválido.");
      return;
    }
    setError("");
    setIsLoading(true);
    //
    setUrl("");
  };

  const handleRemoveJob = (id: string) => {};

  return (
    <main className="bg-gray-900 text-white min-h-screen font-sans p-4 sm:p-6 lg:p-8">
      <div className="max-w-4xl mx-auto">
        <header className="mb-8 flex justify-between items-center">
          <div className="flex items-center gap-4">
            <Youtube className="h-10 w-10 text-red-600" />
            <div>
              <h1 className="text-3xl font-bold tracking-tight bg-gradient-to-r from-red-500 to-red-700 text-transparent bg-clip-text">
                AdVideo
              </h1>
            </div>
          </div>
          <UserMenu user={user} onLogout={logout} />
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
                placeholder="Cole o link do YouTube aqui..."
                className="w-full bg-gray-900 border border-gray-600 rounded-lg py-3 pl-10 pr-4 focus:ring-2 focus:ring-red-500 focus:border-red-500 transition-all placeholder-gray-500"
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
                disabled={isLoading || !url}
                className="w-full sm:w-auto flex-shrink-0 flex items-center justify-center gap-2 bg-red-600 hover:bg-red-700 text-white font-bold py-2.5 px-6 rounded-lg transition-all disabled:bg-red-800 disabled:cursor-not-allowed"
              >
                {isLoading ? (
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
            {/* Recursos de IA ✨ por Gemini. */}
          </p>
        </section>

        {jobs.length > 0 && (
          <section className="mt-8">
            <h2 className="text-2xl font-semibold mb-4 text-gray-300">
              Fila de Processamento
            </h2>
            <div className="space-y-4">
              {jobs.map((job) => (
                <DownloadCard
                  key={job.id}
                  job={job}
                  onRemove={handleRemoveJob}
                  // onGeminiAction={handleGeminiAction}
                />
              ))}
            </div>
          </section>
        )}
      </div>
    </main>
  );
};
