import React, { useState, useEffect } from "react";
import {
  Clapperboard,
  Music,
  Download,
  Loader,
  Trash2,
  Sparkles,
  Copy,
  MessageSquareQuote,
} from "lucide-react";
import { StatusBadge } from "../ui/StatusBadge";

interface Job {
  id: number;
  title?: string;
  status: "queue" | "processing" | "complete" | "error";
  format: string;
  thumbnail?: string;
  downloadUrl?: string;
  completedAt?: number;
  summary?: string;
  tweet?: string;
  isSummarizing?: boolean;
  isGeneratingTweet?: boolean;
}

interface DownloadCardProps {
  job: Job;
  onRemove: (id: number) => void;
  onGeminiAction: (jobId: number, type: "summarize" | "tweet") => void;
}

export const DownloadCard: React.FC<DownloadCardProps> = ({
  job,
  onRemove,
  onGeminiAction,
}) => {
  const [timeLeft, setTimeLeft] = useState("");
  const [copied, setCopied] = useState<string | null>(null);

  useEffect(() => {
    if (job.status !== "complete" || !job.completedAt) return;

    const calc = () => {
      const diff = job.completedAt! + 24 * 60 * 60 * 1000 - Date.now();
      if (diff <= 0) {
        setTimeLeft("Expirado");
        return;
      }
      const h = String(Math.floor((diff / (1000 * 60 * 60)) % 24)).padStart(
        2,
        "0"
      );
      const m = String(Math.floor((diff / 1000 / 60) % 60)).padStart(2, "0");
      const s = String(Math.floor((diff / 1000) % 60)).padStart(2, "0");
      setTimeLeft(`${h}h ${m}m ${s}s`);
    };

    const id = setInterval(calc, 1000);
    return () => clearInterval(id);
  }, [job.status, job.completedAt]);

  const copy = (text: string, type: string) => {
    navigator.clipboard.writeText(text);
    setCopied(type);
    setTimeout(() => setCopied(null), 2000);
  };

  return (
    <div className="bg-gray-800/50 backdrop-blur-sm p-4 rounded-lg shadow-md flex flex-col gap-4 transition-all duration-500 animate-fade-in">
      <div className="flex flex-col md:flex-row items-center gap-4">
        <img
          src={
            job.thumbnail || "https://placehold.co/120x90/1f2937/4b5563?text=YT"
          }
          alt="Thumbnail"
          className="w-32 h-auto object-cover rounded-md flex-shrink-0"
        />
        <div className="flex-grow text-center md:text-left w-full">
          <p className="font-semibold text-gray-200 break-words">
            {job.title || "Carregando..."}
          </p>
          <div className="flex items-center justify-center md:justify-start gap-2 mt-2">
            <StatusBadge status={job.status} />
            {job.format === "video" ? (
              <Clapperboard className="h-4 w-4 text-gray-400" />
            ) : (
              <Music className="h-4 w-4 text-gray-400" />
            )}
          </div>
        </div>
        <div className="flex-shrink-0 flex flex-col items-center gap-2 w-full md:w-auto">
          {job.status === "complete" && (
            <>
              <a
                href={job.downloadUrl || "#"}
                download
                className="w-full md:w-auto flex items-center justify-center gap-2 bg-blue-600 hover:bg-blue-700 text-white font-bold py-2 px-4 rounded-lg transition-colors"
              >
                <Download size={16} />
                <span>Baixar</span>
              </a>
              <p className="text-xs text-yellow-400 font-mono">
                Expira em: {timeLeft}
              </p>
            </>
          )}
          {job.status === "error" && (
            <p className="text-sm text-red-400">Falha.</p>
          )}
          {job.status !== "queue" && job.status !== "processing" && (
            <button
              onClick={() => onRemove(job.id)}
              className="text-gray-500 hover:text-red-500 transition-colors"
            >
              <Trash2 size={16} />
            </button>
          )}
        </div>
      </div>

      {job.status === "complete" && (
        <div className="border-t border-gray-700 pt-4 mt-2">
          <div className="flex flex-col sm:flex-row gap-2">
            <button
              onClick={() => onGeminiAction(job.id, "summarize")}
              disabled={job.isSummarizing || !!job.summary}
              className="flex-1 flex items-center justify-center gap-2 bg-purple-600/20 hover:bg-purple-600/40 text-purple-300 border border-purple-500/50 font-semibold py-2 px-4 rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {job.isSummarizing ? (
                <Loader className="animate-spin" size={16} />
              ) : (
                <Sparkles size={16} />
              )}
              <span>{job.summary ? "Resumo Gerado" : "✨ Resumir"}</span>
            </button>
            <button
              onClick={() => onGeminiAction(job.id, "tweet")}
              disabled={job.isGeneratingTweet || !!job.tweet}
              className="flex-1 flex items-center justify-center gap-2 bg-cyan-600/20 hover:bg-cyan-600/40 text-cyan-300 border border-cyan-500/50 font-semibold py-2 px-4 rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {job.isGeneratingTweet ? (
                <Loader className="animate-spin" size={16} />
              ) : (
                <Sparkles size={16} />
              )}
              <span>{job.tweet ? "Tweet Criado" : "✨ Criar Tweet"}</span>
            </button>
          </div>

          {job.summary && (
            <div className="mt-4 p-4 bg-gray-900/70 rounded-lg animate-fade-in">
              <h4 className="font-semibold text-purple-300 flex items-center gap-2">
                <MessageSquareQuote size={16} />
                Resumo
              </h4>
              <p className="text-gray-300 text-sm mt-2 whitespace-pre-wrap">
                {job.summary}
              </p>
              <button
                onClick={() => copy(job.summary!, "summary")}
                className="mt-2 flex items-center gap-1 text-xs text-gray-400 hover:text-white"
              >
                <Copy size={12} />
                {copied === "summary" ? "Copiado!" : "Copiar"}
              </button>
            </div>
          )}

          {job.tweet && (
            <div className="mt-4 p-4 bg-gray-900/70 rounded-lg animate-fade-in">
              <h4 className="font-semibold text-cyan-300 flex items-center gap-2">
                <Sparkles size={16} />
                Tweet
              </h4>
              <p className="text-gray-300 text-sm mt-2 whitespace-pre-wrap">
                {job.tweet}
              </p>
              <button
                onClick={() => copy(job.tweet!, "tweet")}
                className="mt-2 flex items-center gap-1 text-xs text-gray-400 hover:text-white"
              >
                <Copy size={12} />
                {copied === "tweet" ? "Copiado!" : "Copiar"}
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  );
};
