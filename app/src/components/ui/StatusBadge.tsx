import React from "react";
import { Loader, CheckCircle, AlertTriangle } from "lucide-react";

interface StatusBadgeProps {
  status: "queue" | "processing" | "complete" | "error";
}

export const StatusBadge: React.FC<StatusBadgeProps> = ({ status }) => {
  const config = {
    queue: {
      text: "Na Fila",
      icon: <Loader className="animate-spin h-3 w-3 mr-1" />,
      className: "bg-gray-600 text-gray-200",
    },
    processing: {
      text: "Processando",
      icon: <Loader className="animate-spin h-3 w-3 mr-1" />,
      className: "bg-blue-600 text-white",
    },
    complete: {
      text: "Concluído",
      icon: <CheckCircle className="h-3 w-3 mr-1" />,
      className: "bg-green-600 text-white",
    },
    error: {
      text: "Erro",
      icon: <AlertTriangle className="h-3 w-3 mr-1" />,
      className: "bg-red-600 text-white",
    },
  }[status] || { text: "...", icon: null, className: "" };

  return (
    <div
      className={`flex items-center text-xs font-medium px-2.5 py-0.5 rounded-full ${config.className}`}
    >
      {config.icon}
      <span>{config.text}</span>
    </div>
  );
};
