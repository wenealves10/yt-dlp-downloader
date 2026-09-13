import React, { useState } from "react";
import { Check, Copy, Eye, EyeOff, TriangleAlert } from "lucide-react";

interface Props {
  titulo: string;
  valor: string;
  /** Verdadeiro quando o valor não poderá ser consultado de novo. */
  unicaVez?: boolean;
  descricao?: React.ReactNode;
}

// Caixa de segredo: mostra, esconde e copia.
//
// Existe por causa de um detalhe que decide se a integração vai funcionar: a
// chave de API aparece UMA ÚNICA VEZ, na resposta que a criou. O banco guarda
// só o hash. Se quem está cadastrando fechar a tela sem copiar, não há como
// recuperar — só emitir outra chave. O aviso e o botão de copiar existem para
// que isso não aconteça por descuido.
export const SegredoRevelado: React.FC<Props> = ({
  titulo,
  valor,
  unicaVez = false,
  descricao,
}) => {
  const [visivel, setVisivel] = useState(unicaVez);
  const [copiado, setCopiado] = useState(false);
  const [erroCopia, setErroCopia] = useState("");

  const copiar = async () => {
    setErroCopia("");
    try {
      await navigator.clipboard.writeText(valor);
      setCopiado(true);
      window.setTimeout(() => setCopiado(false), 2000);
    } catch {
      // A área de transferência exige contexto seguro (https ou localhost) e
      // permissão. Quando falha, o valor continua selecionável na tela — mas
      // dizer isso é melhor do que um botão que não faz nada.
      setErroCopia("Não foi possível copiar automaticamente; selecione e copie o texto.");
      setVisivel(true);
    }
  };

  return (
    <div
      className={`rounded-xl border p-4 ${
        unicaVez
          ? "border-amber-700/60 bg-amber-950/30"
          : "border-gray-700 bg-gray-800/60"
      }`}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="text-xs font-medium uppercase tracking-wide text-gray-400">
            {titulo}
          </p>
          {unicaVez && (
            <p className="mt-1 flex items-start gap-1.5 text-xs text-amber-300">
              <TriangleAlert size={14} className="mt-0.5 shrink-0" />
              <span>
                Copie agora. Este valor não aparece de novo — o servidor guarda
                apenas o hash dele.
              </span>
            </p>
          )}
          {descricao && (
            <p className="mt-1 text-xs text-gray-500">{descricao}</p>
          )}
        </div>

        <div className="flex shrink-0 items-center gap-1">
          {!unicaVez && (
            <button
              type="button"
              onClick={() => setVisivel((atual) => !atual)}
              className="rounded-lg p-2 text-gray-400 transition-colors hover:bg-gray-700 hover:text-gray-200"
              aria-label={visivel ? "Esconder" : "Mostrar"}
              title={visivel ? "Esconder" : "Mostrar"}
            >
              {visivel ? <EyeOff size={16} /> : <Eye size={16} />}
            </button>
          )}
          <button
            type="button"
            onClick={copiar}
            className="flex items-center gap-1.5 rounded-lg bg-gray-700 px-3 py-2 text-xs font-medium text-gray-200 transition-colors hover:bg-gray-600"
          >
            {copiado ? <Check size={14} /> : <Copy size={14} />}
            {copiado ? "Copiado" : "Copiar"}
          </button>
        </div>
      </div>

      <p className="mt-3 break-all rounded-lg bg-gray-950/60 px-3 py-2 font-mono text-xs text-gray-100 select-all">
        {visivel ? valor : "•".repeat(Math.min(valor.length, 52))}
      </p>

      {erroCopia && <p className="mt-2 text-xs text-amber-300">{erroCopia}</p>}
    </div>
  );
};
