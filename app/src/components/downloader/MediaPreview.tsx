import React, { useState } from "react";
import {
  ChevronDown,
  Clapperboard,
  Download,
  ExternalLink,
  Eye,
  Loader,
  Music,
  ThumbsUp,
  Users,
  X,
} from "lucide-react";
import type { MediaFormat, ResolvedMedia } from "../../interface/Media";
import {
  formatBytes,
  formatCompact,
  formatDuration,
  formatUploadDate,
} from "./format";

interface Props {
  midia: ResolvedMedia;
  formatoID: string;
  aoTrocarFormato: (id: string) => void;
  aoBaixar: () => void;
  aoDescartar: () => void;
  baixando: boolean;
  bloqueado: boolean;
}

// rotuloFormato descreve a opção sem depender só do número: "1080p" sozinho não
// diz se sai vídeo ou áudio, nem quanto vai ocupar.
function rotuloFormato(formato: MediaFormat): string {
  const partes = [formato.label];
  if (formato.ext) partes.push(formato.ext.toUpperCase());
  if (formato.size_bytes) {
    partes.push(`${formato.size_approximate ? "~" : ""}${formatBytes(formato.size_bytes)}`);
  }
  return partes.join(" · ");
}

// Estatística com ícone. Só aparece quando a plataforma informou o número —
// mostrar "0 visualizações" para um conteúdo sem esse dado seria inventar.
const Estatistica: React.FC<{
  icone: React.ReactNode;
  valor?: number;
  titulo: string;
}> = ({ icone, valor, titulo }) =>
  valor && valor > 0 ? (
    <span className="flex items-center gap-1.5 tabular-nums" title={titulo}>
      {icone}
      {formatCompact(valor)}
    </span>
  ) : null;

export const MediaPreview: React.FC<Props> = ({
  midia,
  formatoID,
  aoTrocarFormato,
  aoBaixar,
  aoDescartar,
  baixando,
  bloqueado,
}) => {
  const [descricaoAberta, setDescricaoAberta] = useState(false);

  const formatoEscolhido = midia.formats.find((item) => item.id === formatoID);
  const ehAudio = formatoEscolhido?.kind === "audio";
  const data = formatUploadDate(midia.upload_date);

  return (
    <article className="mt-4 rounded-xl border border-gray-700 bg-gray-900/60 overflow-hidden animate-fade-in-fast">
      <div className="flex flex-col sm:flex-row gap-4 p-4">
        <div className="relative shrink-0 self-start w-full sm:w-44">
          {midia.thumbnail ? (
            <img
              src={midia.thumbnail}
              alt=""
              loading="lazy"
              className="w-full aspect-video object-cover rounded-lg bg-gray-800"
            />
          ) : (
            <div className="w-full aspect-video rounded-lg bg-gray-800 flex items-center justify-center">
              <Clapperboard className="text-gray-600" size={26} />
            </div>
          )}

          {/* A duração sobre a miniatura é a convenção de todo player, e libera
              a linha de informações para os dados que só existem aqui. */}
          {midia.duration > 0 && (
            <span className="absolute bottom-1.5 right-1.5 px-1.5 py-0.5 rounded bg-black/80 text-white text-xs font-medium tabular-nums">
              {formatDuration(midia.duration)}
            </span>
          )}
        </div>

        <div className="min-w-0 flex-grow">
          <div className="flex items-start gap-3">
            {/* Uma linha só: um título longo não pode empurrar o resto do card.
                O texto completo fica a um hover de distância. */}
            <h2
              className="min-w-0 flex-grow truncate font-semibold text-gray-50 leading-snug"
              title={midia.title}
            >
              {midia.title || "Sem título"}
            </h2>
            <button
              type="button"
              onClick={aoDescartar}
              className="shrink-0 -mt-0.5 p-1 rounded-md text-gray-500 hover:text-white hover:bg-gray-700 transition-colors"
              aria-label="Descartar"
            >
              <X size={16} />
            </button>
          </div>

          {midia.uploader && (
            <div className="mt-2">
              {midia.uploader_url ? (
                // noopener impede que a página aberta alcance esta pela
                // window.opener; a URL já foi validada como http(s) no servidor.
                <a
                  href={midia.uploader_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-1.5 text-sm font-medium text-gray-200 hover:text-white group"
                >
                  <span className="truncate max-w-[16rem]">{midia.uploader}</span>
                  <ExternalLink
                    size={13}
                    className="shrink-0 text-gray-500 group-hover:text-red-400 transition-colors"
                  />
                </a>
              ) : (
                <span className="text-sm font-medium text-gray-200">
                  {midia.uploader}
                </span>
              )}
            </div>
          )}

          <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-gray-400">
            <span className="px-2 py-0.5 rounded-full bg-gray-800 text-gray-300 font-medium">
              {midia.platform_label}
            </span>
            <Estatistica
              icone={<Eye size={13} />}
              valor={midia.view_count}
              titulo="Visualizações"
            />
            <Estatistica
              icone={<ThumbsUp size={13} />}
              valor={midia.like_count}
              titulo="Curtidas"
            />
            <Estatistica
              icone={<Users size={13} />}
              valor={midia.follower_count}
              titulo="Inscritos no canal"
            />
            {data && <span>{data}</span>}
          </div>
        </div>
      </div>

      {midia.description && (
        <div className="px-4 pb-4">
          <p
            className={`text-sm text-gray-400 leading-relaxed whitespace-pre-line ${
              descricaoAberta ? "" : "line-clamp-2"
            }`}
          >
            {midia.description}
          </p>
          {/* O botão só aparece quando há mais para ver; uma descrição de duas
              linhas não ganha um controle inútil. */}
          {midia.description.length > 140 && (
            <button
              type="button"
              onClick={() => setDescricaoAberta((aberta) => !aberta)}
              className="mt-1 inline-flex items-center gap-1 text-xs text-gray-400 hover:text-gray-200 transition-colors"
            >
              {descricaoAberta ? "Mostrar menos" : "Mostrar mais"}
              <ChevronDown
                size={12}
                className={`transition-transform ${descricaoAberta ? "rotate-180" : ""}`}
              />
            </button>
          )}
        </div>
      )}

      <div className="border-t border-gray-700 bg-gray-900/40 p-4 flex flex-col sm:flex-row sm:items-end gap-3">
        {midia.formats.length > 0 ? (
          <label className="flex-grow min-w-0">
            <span className="block text-xs font-medium text-gray-400 mb-1.5">
              Qualidade
            </span>
            <select
              value={formatoID}
              onChange={(evento) => aoTrocarFormato(evento.target.value)}
              className="w-full bg-gray-900 border border-gray-600 rounded-lg py-2.5 px-3 text-gray-100 focus:ring-2 focus:ring-red-500 focus:border-red-500 transition-all"
            >
              {midia.formats.map((formato) => (
                <option key={formato.id} value={formato.id}>
                  {rotuloFormato(formato)}
                </option>
              ))}
            </select>
          </label>
        ) : (
          <p className="flex-grow text-sm text-gray-400">
            Esta plataforma não informa as qualidades disponíveis. Vamos baixar na
            melhor que estiver acessível.
          </p>
        )}

        <button
          type="button"
          onClick={aoBaixar}
          disabled={baixando || bloqueado}
          className="shrink-0 flex items-center justify-center gap-2 bg-red-600 hover:bg-red-700 text-white font-semibold py-2.5 px-6 rounded-lg transition-colors disabled:bg-red-800 disabled:cursor-not-allowed"
        >
          {baixando ? (
            <>
              <Loader className="animate-spin" size={18} /> Iniciando...
            </>
          ) : (
            <>
              {ehAudio ? <Music size={18} /> : <Download size={18} />}
              Baixar
            </>
          )}
        </button>
      </div>
    </article>
  );
};
