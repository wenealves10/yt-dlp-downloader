import React, { useState } from "react";
import { AlertTriangle, Link as LinkIcon, Loader, Search } from "lucide-react";
import { useAuth } from "../../hooks/useAuth";
import { createMediaDownload, resolveMedia } from "../../api/mediaApi";
import type { ResolvedMedia } from "../../interface/Media";
import { MediaError } from "../../interface/Media";
import { MediaPreview } from "./MediaPreview";

interface Props {
  aoCriar: () => void;
  bloqueado: boolean;
}

// dicaPorCodigo transforma o erro em uma orientação prática. O código é
// estável; a mensagem do servidor pode mudar sem quebrar isto.
function dicaPorCodigo(codigo: string): string {
  switch (codigo) {
    case "unsupported_platform":
      return "Confira se o link é de um vídeo público de uma plataforma suportada.";
    case "content_private":
    case "auth_required":
      return "Só conseguimos baixar conteúdo público.";
    case "live_content":
      return "Tente novamente quando a transmissão terminar.";
    case "geo_blocked":
      return "O conteúdo está bloqueado para a região do servidor.";
    case "rate_limited":
      return "A plataforma pediu uma pausa. Tente de novo em alguns minutos.";
    case "invalid_url":
      return "Cole o endereço completo do vídeo.";
    default:
      return "";
  }
}

export const MediaResolver: React.FC<Props> = ({ aoCriar, bloqueado }) => {
  const { token } = useAuth();

  const [url, setUrl] = useState("");
  const [resolvendo, setResolvendo] = useState(false);
  const [criando, setCriando] = useState(false);
  const [midia, setMidia] = useState<ResolvedMedia | null>(null);
  const [formatoID, setFormatoID] = useState("");
  const [erro, setErro] = useState<{ mensagem: string; dica: string } | null>(null);

  const tratarErro = (falha: unknown) => {
    const codigo = falha instanceof MediaError ? falha.code : "unknown";
    setErro({
      mensagem: falha instanceof Error ? falha.message : "Algo deu errado.",
      dica: dicaPorCodigo(codigo),
    });
  };

  const resolver = async (evento: React.FormEvent) => {
    evento.preventDefault();
    if (!token || !url.trim()) return;

    setResolvendo(true);
    setErro(null);
    setMidia(null);
    try {
      const resultado = await resolveMedia(token)(url.trim());
      setMidia(resultado);
      // Pré-seleciona a melhor qualidade: é a escolha da maioria, e a lista já
      // vem ordenada da maior resolução para a menor.
      setFormatoID(resultado.formats[0]?.id ?? "");
    } catch (falha) {
      tratarErro(falha);
    } finally {
      setResolvendo(false);
    }
  };

  const baixar = async () => {
    if (!token || !midia) return;

    const formato = midia.formats.find((item) => item.id === formatoID);

    setCriando(true);
    setErro(null);
    try {
      await createMediaDownload(token)({
        url: midia.url,
        format_id: formatoID || undefined,
        kind: formato?.kind ?? "video",
      });
      // Some com a prévia: o download já está na lista abaixo, e manter o card
      // sugeriria que ele ainda não começou.
      setMidia(null);
      setUrl("");
      aoCriar();
    } catch (falha) {
      tratarErro(falha);
    } finally {
      setCriando(false);
    }
  };

  return (
    <section className="bg-gray-800 p-6 rounded-xl shadow-2xl border border-gray-700">
      <form onSubmit={resolver}>
        <div className="relative mb-4">
          <LinkIcon className="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-gray-500" />
          <input
            type="text"
            value={url}
            onChange={(evento) => {
              setUrl(evento.target.value);
              setErro(null);
            }}
            disabled={bloqueado}
            placeholder="Cole o link do vídeo (YouTube, TikTok, Instagram, X...)"
            className={`w-full bg-gray-900 border border-gray-600 rounded-lg py-3 pl-10 pr-4 focus:ring-2 focus:ring-red-500 focus:border-red-500 transition-all placeholder-gray-400 ${
              bloqueado ? "opacity-50 cursor-not-allowed" : ""
            }`}
          />
        </div>

        {erro && (
          <div className="mb-4 flex items-start gap-3 bg-red-900/30 border border-red-700 text-red-200 rounded-lg p-3 text-sm">
            <AlertTriangle size={18} className="mt-0.5 shrink-0" />
            <span>
              {erro.mensagem}
              {erro.dica && (
                <span className="block text-red-300/70 mt-0.5">{erro.dica}</span>
              )}
            </span>
          </div>
        )}

        {!midia && (
          <button
            type="submit"
            disabled={resolvendo || !url.trim() || bloqueado}
            className="w-full flex items-center justify-center gap-2 bg-red-600 hover:bg-red-700 text-white font-bold py-2.5 px-6 rounded-lg transition-all disabled:bg-red-800 disabled:cursor-not-allowed"
          >
            {resolvendo ? (
              <>
                <Loader className="animate-spin" size={20} /> Buscando informações...
              </>
            ) : (
              <>
                <Search size={20} /> Buscar
              </>
            )}
          </button>
        )}
      </form>

      {midia && (
        <MediaPreview
          midia={midia}
          formatoID={formatoID}
          aoTrocarFormato={setFormatoID}
          aoBaixar={baixar}
          aoDescartar={() => {
            setMidia(null);
            setErro(null);
          }}
          baixando={criando}
          bloqueado={bloqueado}
        />
      )}
    </section>
  );
};
