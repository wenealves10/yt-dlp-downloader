import React, { useCallback, useEffect, useRef, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import RFB from "@novnc/novnc/lib/rfb.js";
import {
  AlertTriangle,
  CheckCircle,
  Loader,
  PowerOff,
  RefreshCw,
  ShieldCheck,
} from "lucide-react";
import { AdminHeader } from "./AdminHeader";
import { useAuth } from "../../hooks/useAuth";
import {
  buildBrowserSocketURL,
  checkYoutubeAccount,
  closeYoutubeBrowser,
  openYoutubeBrowser,
} from "../../api/adminYoutube";
import type {
  BrowserSession,
  YoutubeAccount,
} from "../../interface/YoutubeAccount";
import { AccountStatusBadge } from "./AccountStatusBadge";

type ConnectionState = "idle" | "opening" | "connecting" | "connected" | "closed";

// Intervalo entre as verificações automáticas enquanto o administrador está na
// tela do navegador. A verificação reaproveita o navegador já aberto, então o
// custo é uma requisição ao YouTube; ela para assim que a sessão é detectada.
const autoCheckIntervalMs = 10_000;

export const RemoteBrowserPage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const { token } = useAuth();
  const navigate = useNavigate();

  const screenRef = useRef<HTMLDivElement>(null);
  const rfbRef = useRef<RFB | null>(null);

  const [state, setState] = useState<ConnectionState>("idle");
  const [account, setAccount] = useState<YoutubeAccount | null>(null);
  const [error, setError] = useState("");
  const [isBusy, setBusy] = useState(false);

  // Evita que duas verificações (a automática e a manual) corram juntas.
  const checkingRef = useRef(false);

  // disconnect é idempotente: usado tanto no unmount quanto antes de reconectar.
  const disconnect = useCallback(() => {
    if (rfbRef.current) {
      rfbRef.current.disconnect();
      rfbRef.current = null;
    }
  }, []);

  const connect = useCallback(
    (session: BrowserSession) => {
      if (!screenRef.current) return;

      disconnect();
      setState("connecting");

      // O ticket é de uso único e some da URL assim que o WebSocket é aberto.
      const url = buildBrowserSocketURL(session.ws_path, session.ticket);
      const rfb = new RFB(screenRef.current, url, {
        credentials: { password: session.vnc_password },
      });

      rfb.scaleViewport = true;
      rfb.resizeSession = false;
      rfb.background = "#111827";
      rfb.focusOnClick = true;

      rfb.addEventListener("connect", () => setState("connected"));
      rfb.addEventListener("disconnect", () => {
        setState("closed");
        rfbRef.current = null;
      });
      rfb.addEventListener("securityfailure", () => {
        setError("O navegador remoto recusou a conexão.");
        setState("closed");
      });

      rfbRef.current = rfb;
    },
    [disconnect]
  );

  const openSession = useCallback(async () => {
    if (!id || !token) return;

    setError("");
    setState("opening");
    try {
      const session = await openYoutubeBrowser(token)(id);
      setAccount(session.account);
      connect(session);
    } catch (openError) {
      setError(
        openError instanceof Error
          ? openError.message
          : "Não foi possível abrir o navegador remoto."
      );
      setState("closed");
    }
  }, [connect, id, token]);

  useEffect(() => {
    openSession();
    return disconnect;
    // Abrir a sessão uma única vez ao entrar na tela; reconexões são manuais.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // runCheck é compartilhado pelo botão e pela verificação automática. O modo
  // silencioso não mexe no estado dos botões nem mostra erro: uma falha
  // isolada de rede durante o login não deve poluir a tela.
  const runCheck = useCallback(
    async (silent: boolean) => {
      if (!id || !token || checkingRef.current) return;

      checkingRef.current = true;
      if (!silent) {
        setBusy(true);
        setError("");
      }
      try {
        setAccount(await checkYoutubeAccount(token)(id));
      } catch (checkError) {
        if (!silent) {
          setError(
            checkError instanceof Error
              ? checkError.message
              : "Não foi possível verificar a sessão."
          );
        }
      } finally {
        checkingRef.current = false;
        if (!silent) setBusy(false);
      }
    },
    [id, token]
  );

  // Detecta o login sozinho: enquanto a tela está conectada e a conta ainda não
  // foi autenticada, o painel verifica a sessão periodicamente e para assim que
  // ela é reconhecida.
  useEffect(() => {
    if (state !== "connected") return;
    if (account?.status === "AUTHENTICATED") return;

    const timer = setInterval(() => {
      if (document.visibilityState === "visible") {
        runCheck(true);
      }
    }, autoCheckIntervalMs);

    return () => clearInterval(timer);
  }, [state, account?.status, runCheck]);

  // Fechar o navegador preserva o perfil: a sessão continua salva em disco.
  const handleClose = async () => {
    if (!id || !token) return;
    setBusy(true);
    setError("");
    try {
      disconnect();
      await closeYoutubeBrowser(token)(id);
      navigate("/admin/youtube/accounts");
    } catch (closeError) {
      setError(
        closeError instanceof Error
          ? closeError.message
          : "Não foi possível fechar o navegador."
      );
      setBusy(false);
    }
  };

  return (
    <main className="bg-gray-900 text-white min-h-screen font-sans p-4 sm:p-6 lg:p-8">
      <div className="max-w-7xl mx-auto">
        <AdminHeader
          breadcrumb={["YouTube", "Contas", account?.label ?? "Navegador"]}
          backTo="/admin/youtube/accounts"
          backLabel="Contas"
        />

        <section className="mb-4 flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-3">
            <h1 className="text-xl font-semibold text-gray-100">
              Navegador remoto
            </h1>
            {account && <AccountStatusBadge status={account.status} />}
            {state === "connected" && (
              <span className="text-xs text-green-400">conectado</span>
            )}
            {(state === "opening" || state === "connecting") && (
              <span className="flex items-center gap-1 text-xs text-blue-300">
                <Loader className="animate-spin" size={12} />
                {state === "opening" ? "abrindo navegador" : "conectando"}
              </span>
            )}
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <button
              type="button"
              onClick={openSession}
              disabled={state === "opening" || state === "connecting"}
              className="flex items-center gap-2 border border-gray-700 text-gray-300 hover:bg-gray-800 disabled:opacity-50 text-sm px-4 py-2 rounded-lg transition-colors"
            >
              <RefreshCw size={16} />
              Reconectar
            </button>
            <button
              type="button"
              onClick={() => runCheck(false)}
              disabled={isBusy}
              className="flex items-center gap-2 border border-gray-700 text-gray-300 hover:bg-gray-800 disabled:opacity-50 text-sm px-4 py-2 rounded-lg transition-colors"
            >
              {isBusy ? (
                <Loader className="animate-spin" size={16} />
              ) : (
                <ShieldCheck size={16} />
              )}
              Verificar sessão
            </button>
            <button
              type="button"
              onClick={handleClose}
              disabled={isBusy}
              className="flex items-center gap-2 bg-red-600 hover:bg-red-700 disabled:bg-red-800 text-white text-sm font-medium px-4 py-2 rounded-lg transition-colors"
            >
              <PowerOff size={16} />
              Fechar navegador
            </button>
          </div>
        </section>

        {account?.status === "AUTHENTICATED" ? (
          <div className="mb-4 flex items-start gap-3 bg-green-900/30 border border-green-700 text-green-200 rounded-lg p-4 text-sm">
            <CheckCircle size={18} className="mt-0.5 shrink-0" />
            <span>
              Sessão autenticada e salva. Pode fechar o navegador: os downloads
              já vão usar esta conta.
            </span>
          </div>
        ) : (
          <p className="mb-4 text-sm text-gray-400">
            Faça o login normalmente na tela abaixo. O painel detecta a sessão
            sozinho assim que o login terminar; depois disso a sessão continua
            salva mesmo com o navegador fechado.
          </p>
        )}

        {error && (
          <div className="mb-4 flex items-start gap-3 bg-red-900/30 border border-red-700 text-red-200 rounded-lg p-4 text-sm">
            <AlertTriangle size={18} className="mt-0.5 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        <div className="bg-black border border-gray-700 rounded-xl overflow-hidden">
          <div
            ref={screenRef}
            className="w-full"
            style={{ height: "min(75vh, 900px)" }}
          />
        </div>

        {state === "closed" && !error && (
          <p className="mt-4 text-sm text-gray-400">
            A conexão com o navegador foi encerrada. Use "Reconectar" para abrir
            novamente.
          </p>
        )}
      </div>
    </main>
  );
};
