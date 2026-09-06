import React, { useEffect, useState } from "react";
import { UserMenu } from "../user/UserMenu";
import { DownloadCard } from "./DownloadCard";
import { useAuth } from "../../hooks/useAuth";
import { useQuery } from "@tanstack/react-query";
import { getData, startFileDownload } from "../../api/getData";
import { bucketHost } from "../../constants/config";
import { DownloadCounter } from "./DownloadCounter";
import { useDeleteDownloadMutation } from "../../hooks/useDownloadMutation";
import { useDownloads } from "../../hooks/useDownload";
import { MediaResolver } from "./MediaResolver";
import { resolverThumbnail } from "./format";
import { cancelDownload } from "../../api/mediaApi";
import type { Download as DownloadType } from "../../interface/Download";
import type { DownloadProgress } from "../../interface/Media";
const apiUrl = import.meta.env.VITE_API_URL;

interface Job {
  id: string;
  title?: string;
  status: "queue" | "processing" | "complete" | "expired" | "error" | "canceled";
  format: string;
  thumbnail?: string;
  durationSeconds?: number;
  completedAt?: number;
  expiresAt?: string | null;
  summary?: string;
  tweet?: string;
  isSummarizing?: boolean;
  isGeneratingTweet?: boolean;
  platform?: string;
  uploader?: string;
  quality?: string;
  sizeBytes?: number;
  progress?: DownloadProgress;
  errorMessage?: string;
  errorDetail?: string;
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
    case "CANCELED":
      return "canceled";
    default:
      return "queue";
  }
}

export const DownloaderPage: React.FC = () => {
  const { user, logout, token } = useAuth();
  const [jobs, setJobs] = useState<Job[]>([]);
  const [error, setError] = useState("");
  const [downloadingJobID, setDownloadingJobID] = useState<string | null>(null);
  const [page] = useState(1);
  const [perPage] = useState(10);
  const { remaining, unlimited, refetch } = useDownloads();
  const deleteDownload = useDeleteDownloadMutation();
  const semSaldo = !unlimited && remaining === 0;

  const downloadsQuery = useQuery({
    queryKey: ["downloads", page, perPage],
    queryFn: () => getData(token || "")(perPage, page),
    refetchOnWindowFocus: false,
    enabled: !!token,
  });


  useEffect(() => {
    if (downloadsQuery.data?.downloads) {
      const mapped: Job[] = downloadsQuery.data.downloads.map((item) => ({
        id: item.id,
        title: item.title,
        status: convertStatus(item.status),
        durationSeconds: item.duration_seconds,
        format: item.format.toLowerCase(),
        thumbnail: resolverThumbnail(item.thumbnail_url, bucketHost),
        completedAt: new Date(item.created_at).getTime(),
        expiresAt: item.expires_at,
        platform: item.platform_label,
        uploader: item.uploader,
        quality: item.quality_label,
        sizeBytes: item.file_size_bytes,
        errorMessage: item.error_message,
        errorDetail: item.error_detail,
      }));

      // O histórico não traz o progresso ao vivo — ele chega por SSE. Trocar a
      // lista inteira aqui apagaria a barra de um download em andamento toda
      // vez que a consulta revalidasse.
      setJobs((anteriores) => {
        const progressoAtual = new Map(
          anteriores
            .filter((job) => job.progress)
            .map((job) => [job.id, job.progress])
        );
        return mapped.map((job) =>
          progressoAtual.has(job.id)
            ? { ...job, progress: progressoAtual.get(job.id) }
            : job
        );
      });
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
          const anterior = updatedJobs[existingIndex];
          updatedJobs[existingIndex] = {
            ...anterior,
            ...data,
            status: convertStatus(data.status),
            thumbnail:
              resolverThumbnail(data.thumbnail_url, bucketHost) ||
              anterior.thumbnail,
            expiresAt: data.expires_at || anterior.expiresAt,
            platform: data.platform_label || data.platform || anterior.platform,
            // O evento de conclusão não traz progresso; manter o último
            // conhecido evita a barra sumir e voltar durante a transição.
            progress: data.progress ?? anterior.progress,
            errorMessage: data.error_message || anterior.errorMessage,
            errorDetail: anterior.errorDetail,
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

  const handleCancel = async (id: string) => {
    if (!token) return;
    setError("");

    // O card muda na hora. O worker leva até 2 s para perceber o pedido, e
    // deixar o botão "Cancelar" aceso nesse intervalo fazia parecer que o
    // clique não tinha funcionado.
    setJobs((prev) =>
      prev.map((job) =>
        job.id === id && (job.status === "queue" || job.status === "processing")
          ? { ...job, status: "canceled", progress: undefined }
          : job
      )
    );

    try {
      await cancelDownload(token)(id);
    } catch (falha) {
      // A API não recusa mais um cancelamento por corrida de status; se algo
      // falhou aqui foi de verdade, e a lista precisa voltar ao estado real.
      setError(falha instanceof Error ? falha.message : "Não foi possível cancelar.");
      downloadsQuery.refetch();
    }
  };

  const handleRemoveJob = (id: string) => {
    deleteDownload.mutate(id);
    setJobs((prev) => prev.filter((job) => job.id !== id));
  };

  const handleFileDownload = async (id: string) => {
    if (!token) {
      setError("Sua sessão expirou. Entre novamente para baixar o arquivo.");
      return;
    }

    setError("");
    setDownloadingJobID(id);
    try {
      await startFileDownload(id, token);
    } catch (error) {
      setError(
        error instanceof Error ? error.message : "Não foi possível iniciar o download."
      );
    } finally {
      setDownloadingJobID(null);
    }
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

        <MediaResolver
          bloqueado={semSaldo}
          aoCriar={() => {
            downloadsQuery.refetch();
            refetch();
          }}
        />

        {error && (
          <p className="mt-4 text-sm text-red-400">{error}</p>
        )}

        <p className="text-xs text-center text-gray-500 mt-4">
          Arquivos disponíveis por 24h.
        </p>

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
                  onDownload={handleFileDownload}
                  onRemove={handleRemoveJob}
                  onCancel={handleCancel}
                  isDownloading={downloadingJobID === job.id}
                  mostrarDiagnostico={user?.role === "super_admin"}
                />
              ))}
            </div>
          </section>
        )}
      </div>
    </main>
  );
};
