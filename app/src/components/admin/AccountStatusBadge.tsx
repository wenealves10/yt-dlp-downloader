import React from "react";
import {
  AlertTriangle,
  CheckCircle,
  CircleSlash,
  Loader,
  LogIn,
  MonitorPlay,
  MonitorOff,
} from "lucide-react";
import type {
  BrowserState,
  YoutubeAccountStatus,
} from "../../interface/YoutubeAccount";

const statusConfig: Record<
  YoutubeAccountStatus,
  { text: string; icon: React.ReactNode; className: string }
> = {
  NOT_CONFIGURED: {
    text: "Não configurada",
    icon: <CircleSlash className="h-3 w-3 mr-1" />,
    className: "bg-gray-600 text-gray-200",
  },
  AWAITING_LOGIN: {
    text: "Aguardando login",
    icon: <LogIn className="h-3 w-3 mr-1" />,
    className: "bg-amber-600 text-white",
  },
  AUTHENTICATED: {
    text: "Autenticada",
    icon: <CheckCircle className="h-3 w-3 mr-1" />,
    className: "bg-green-600 text-white",
  },
  REQUIRES_AUTH: {
    text: "Requer autenticação",
    icon: <AlertTriangle className="h-3 w-3 mr-1" />,
    className: "bg-yellow-600 text-white",
  },
  DISABLED: {
    text: "Desativada",
    icon: <CircleSlash className="h-3 w-3 mr-1" />,
    className: "bg-gray-700 text-gray-300",
  },
  ERROR: {
    text: "Erro",
    icon: <AlertTriangle className="h-3 w-3 mr-1" />,
    className: "bg-red-600 text-white",
  },
};

export const AccountStatusBadge: React.FC<{ status: YoutubeAccountStatus }> = ({
  status,
}) => {
  const config = statusConfig[status] ?? statusConfig.NOT_CONFIGURED;

  return (
    <div
      className={`inline-flex items-center text-xs font-medium px-2.5 py-0.5 rounded-full ${config.className}`}
    >
      {config.icon}
      <span>{config.text}</span>
    </div>
  );
};

const browserConfig: Record<
  BrowserState,
  { text: string; icon: React.ReactNode; className: string }
> = {
  STOPPED: {
    text: "Navegador fechado",
    icon: <MonitorOff className="h-3 w-3 mr-1" />,
    className: "bg-gray-700 text-gray-300",
  },
  STARTING: {
    text: "Abrindo navegador",
    icon: <Loader className="animate-spin h-3 w-3 mr-1" />,
    className: "bg-blue-600 text-white",
  },
  RUNNING: {
    text: "Navegador ativo",
    icon: <MonitorPlay className="h-3 w-3 mr-1" />,
    className: "bg-blue-600 text-white",
  },
  ERROR: {
    text: "Falha no navegador",
    icon: <AlertTriangle className="h-3 w-3 mr-1" />,
    className: "bg-red-600 text-white",
  },
};

export const BrowserStateBadge: React.FC<{ state: BrowserState }> = ({
  state,
}) => {
  const config = browserConfig[state] ?? browserConfig.STOPPED;

  return (
    <div
      className={`inline-flex items-center text-xs font-medium px-2.5 py-0.5 rounded-full ${config.className}`}
    >
      {config.icon}
      <span>{config.text}</span>
    </div>
  );
};
