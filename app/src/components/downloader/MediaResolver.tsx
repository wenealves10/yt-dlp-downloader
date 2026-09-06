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

// dicaPorCodigo transforma o erro em uma orientação prática.
//
// Toda frase aqui é lida pelo CLIENTE FINAL, então nenhuma pode descrever a
// nossa infraestrutura: nada de servidor, provider, sessão ou plataforma
// recusando alguma coisa. O que sobra é o que a pessoa pode fazer a respeito —
// e quando não há nada que ela possa fazer, a dica fica vazia em vez de
// inventar uma explicação.
//
// Os códigos de infraestrutura chegam colapsados em "unavailable" para quem não
// é super admin; a distinção entre eles é nossa.
function dicaPorCodigo(codigo: string): string {
  switch (codigo) {
    case "unsupported_platform":
      return "Confira se o link é de um vídeo público de um site suportado.";
    case "content_private":
    case "auth_required":
      return "Só é possível baixar conteúdo público.";
    case "live_content":
      return "Tente novamente quando a transmissão terminar.";
    case "rate_limited":
      return "Tente de novo em alguns minutos.";
    case "invalid_url":
      return "Cole o endereço completo do vídeo.";
    case "format_unavailable":
      return "Escolha outra qualidade.";
    default:
      return "";
  }
}

export const MediaResolver: React.FC<Props> = ({ aoCriar, bloqueado }) => {
  const { token, user } = useAuth();
  // Segunda barreira. A API já só envia estes campos ao super admin; a tela
  // também não os desenha para mais ninguém, para que um erro futuro de um lado
  // não vire vazamento sozinho.
  const superAdmin = user?.role === "super_admin";

  const [url, setUrl] = useState("");
  const [resolvendo, setResolvendo] = useState(false);
  const [criando, setCriando] = useState(false);
  const [midia, setMidia] = useState<ResolvedMedia | null>(null);
  const [formatoID, setFormatoID] = useState("");
  const [erro, setErro] = useState<{
    mensagem: string;
    dica: string;
    detalhe?: string;
    sessao?: string;
  } | null>(null);

  const tratarErro = (falha: unknown) => {
    const midiaErro = falha instanceof MediaError ? falha : null;
    setErro({
      mensagem: falha instanceof Error ? falha.message : "Algo deu errado.",
      dica: dicaPorCodigo(midiaErro?.code ?? "unknown"),
      // Só chegam para o super admin; para os demais a API nem envia.
      detalhe: midiaErro?.detail,
      sessao: midiaErro?.session,
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
            <span className="min-w-0">
              {erro.mensagem}
              {erro.dica && (
                <span className="block text-red-300/70 mt-0.5">{erro.dica}</span>
              )}
              {superAdmin && erro.sessao && (
                <span className="block text-red-300/60 mt-1.5 text-xs">
                  Sessão: {erro.sessao}
                </span>
              )}
              {superAdmin && erro.detalhe && (
                <span className="block mt-1.5 text-xs font-mono text-red-300/60 break-words">
                  {erro.detalhe}
                </span>
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
