import React, { useState, useEffect } from "react";
import { Button, Modal, ModalBody } from "flowbite-react";
import {
  Clapperboard,
  Music,
  Download,
  Trash2,
  X,
  TriangleAlert,
} from "lucide-react";
import { StatusBadge } from "../ui/StatusBadge";

interface Job {
  id: string;
  title?: string;
  status: "queue" | "processing" | "complete" | "expired" | "error";
  format: string;
  thumbnail?: string;
  downloadUrl?: string;
  completedAt?: number;
  durationSeconds?: number;
  summary?: string;
  tweet?: string;
  expiresAt?: string | null;
  isSummarizing?: boolean;
  isGeneratingTweet?: boolean;
}

export function formatSeconds(seconds: number): string {
  const h = Math.floor(seconds / 3600)
    .toString()
    .padStart(2, "0");
  const m = Math.floor((seconds % 3600) / 60)
    .toString()
    .padStart(2, "0");
  const s = Math.floor(seconds % 60)
    .toString()
    .padStart(2, "0");

  return `${h}:${m}:${s}`;
}

interface DownloadCardProps {
  job: Job;
  onRemove: (id: string) => void;
}

export const DownloadCard: React.FC<DownloadCardProps> = ({
  job,
  onRemove,
}) => {
  const [timeLeft, setTimeLeft] = useState("");
  const [openModal, setOpenModal] = useState(false);

  useEffect(() => {
    if (job.status !== "complete" || !job.expiresAt) return;

    const calc = () => {
      const expireTs = new Date(job.expiresAt!).getTime();
      const diff = expireTs - Date.now();

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

    calc(); // chamada imediata
    const id = setInterval(calc, 1000);
    return () => clearInterval(id);
  }, [job.status, job.expiresAt]);

  return (
    <div className="bg-gray-800/50 backdrop-blur-sm p-4 rounded-lg shadow-md flex flex-col gap-4 transition-all duration-500 animate-fade-in">
      <div className="flex flex-col md:flex-row items-center gap-4">
        <img
          src={
            job?.thumbnail ||
            "https://placehold.co/120x90/1f2937/4b5563?text=Banner"
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
            {job.format === "mp4" ? (
              <Clapperboard className="h-4 w-4 text-gray-400" />
            ) : (
              <Music className="h-4 w-4 text-gray-400" />
            )}
            {job.durationSeconds && (
              <span className="text-sm text-gray-400">
                {formatSeconds(job.durationSeconds)}
              </span>
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
              onClick={() => setOpenModal(true)}
              className="text-gray-500 hover:text-red-500 transition-colors"
            >
              <Trash2 size={16} />
            </button>
          )}
          <Modal
            show={openModal}
            size="md"
            onClose={() => setOpenModal(false)}
            popup
          >
            <ModalBody className="bg-gray-800 p-6 rounded-md relative">
              <Button
                color="alternative"
                className="absolute top-2 right-2"
                onClick={() => setOpenModal(false)}
              >
                <X className="h-5 w-5" />
              </Button>
              <div className="text-center">
                <TriangleAlert className="mx-auto mb-4 h-14 w-14 text-gray-400 dark:text-gray-200" />
                <h3 className="mb-5 text-lg font-normal text-gray-500 dark:text-gray-400">
                  Tem certeza que deseja excluir este download?
                  <br />
                  Esta ação não pode ser desfeita.
                </h3>
                <div className="flex justify-center gap-4">
                  <Button
                    color="red"
                    onClick={() => {
                      onRemove(job.id);
                      setOpenModal(false);
                    }}
                  >
                    Sim, tenho certeza
                  </Button>
                  <Button
                    color="alternative"
                    onClick={() => setOpenModal(false)}
                  >
                    Não, cancelar
                  </Button>
                </div>
              </div>
            </ModalBody>
          </Modal>
        </div>
      </div>
    </div>
  );
};
