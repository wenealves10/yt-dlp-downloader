import React, { useMemo, useRef, useState } from "react";
import type { SeriesPoint } from "../../../interface/Admin";
import { formatBytes, formatNumber, rotuloBucket } from "../format";

// Paleta validada contra a superfície #1f2937 (o card): banda de luminosidade,
// piso de croma, separação para daltonismo e contraste. Concluídos e falhas
// nunca se distinguem só pela cor — a legenda e o tooltip nomeiam as duas.
const COR_CONCLUIDO = "#3987e5";
const COR_FALHA = "#d95926";

// Espaço para os rótulos dos eixos. Fora dele fica só a área de plotagem.
const MARGEM = { topo: 12, direita: 8, base: 26, esquerda: 44 };
const ALTURA = 260;
const LARGURA = 760;

interface Props {
  series: SeriesPoint[];
  granularity: string;
}

type Metrica = "downloads" | "volume";

export const DownloadsChart: React.FC<Props> = ({ series, granularity }) => {
  const [metrica, setMetrica] = useState<Metrica>("downloads");
  const [ativo, setAtivo] = useState<number | null>(null);
  const svgRef = useRef<SVGSVGElement>(null);

  const larguraPlot = LARGURA - MARGEM.esquerda - MARGEM.direita;
  const alturaPlot = ALTURA - MARGEM.topo - MARGEM.base;

  const { maximo, marcas } = useMemo(() => {
    const valores = series.map((ponto) =>
      metrica === "downloads" ? ponto.total : ponto.transferred_bytes
    );
    const pico = Math.max(...valores, 0);

    if (metrica === "volume") {
      // Bytes já são formatados em unidades legíveis; um teto proporcional
      // basta.
      const teto = pico === 0 ? 1 : pico * 1.15;
      return {
        maximo: teto,
        marcas: [0, 0.25, 0.5, 0.75, 1].map((fracao) => teto * fracao),
      };
    }

    // Contagem é inteira: um eixo marcado em "9,2 downloads" descreve algo que
    // não existe. O passo é arredondado para 1, 2, 5 ou 10 vezes uma potência
    // de dez, que são os intervalos que a pessoa lê sem precisar calcular.
    const alvo = Math.max(1, Math.ceil(pico / 4));
    const magnitude = Math.pow(10, Math.floor(Math.log10(alvo)));
    const passo =
      [1, 2, 5, 10].map((fator) => fator * magnitude).find((valor) => valor >= alvo) ??
      magnitude * 10;
    const teto = Math.max(passo * 4, passo);

    const inteiras: number[] = [];
    for (let valor = 0; valor <= teto; valor += passo) inteiras.push(valor);
    return { maximo: teto, marcas: inteiras };
  }, [series, metrica]);

  const larguraFaixa = series.length > 0 ? larguraPlot / series.length : 0;
  // Barras finas com respiro entre elas; o teto de 26px evita uma barra
  // gigante quando o intervalo tem poucos pontos.
  const larguraBarra = Math.max(2, Math.min(26, larguraFaixa * 0.62));

  const escalaY = (valor: number) => alturaPlot - (valor / maximo) * alturaPlot;

  const aoMover = (evento: React.MouseEvent<SVGSVGElement>) => {
    if (!svgRef.current || series.length === 0) return;
    const caixa = svgRef.current.getBoundingClientRect();
    // A escala do viewBox difere da largura renderizada; converter é o que faz
    // o alvo do mouse bater com a barra sob o cursor.
    const x = ((evento.clientX - caixa.left) / caixa.width) * LARGURA - MARGEM.esquerda;
    const indice = Math.floor(x / larguraFaixa);
    setAtivo(indice >= 0 && indice < series.length ? indice : null);
  };

  const rotuloValor = (valor: number) =>
    metrica === "downloads" ? formatNumber(Math.round(valor)) : formatBytes(valor);

  const pontoAtivo = ativo !== null ? series[ativo] : null;

  return (
    <section className="bg-gray-800 border border-gray-700 rounded-xl p-5">
      <header className="flex flex-wrap items-start justify-between gap-4 mb-4">
        <div>
          <h2 className="text-base font-semibold text-gray-100">
            {metrica === "downloads" ? "Downloads no período" : "Volume transferido"}
          </h2>
          {/* Legenda sempre presente: são duas séries. */}
          {metrica === "downloads" ? (
            <div className="flex items-center gap-4 mt-2 text-xs text-gray-400">
              <span className="flex items-center gap-1.5">
                <span
                  className="w-2.5 h-2.5 rounded-sm"
                  style={{ background: COR_CONCLUIDO }}
                />
                Concluídos
              </span>
              <span className="flex items-center gap-1.5">
                <span
                  className="w-2.5 h-2.5 rounded-sm"
                  style={{ background: COR_FALHA }}
                />
                Falhas
              </span>
            </div>
          ) : (
            <p className="mt-2 text-xs text-gray-400">
              Soma dos arquivos entregues em cada período.
            </p>
          )}
        </div>

        <div className="flex items-stretch gap-1 p-1 bg-gray-900 rounded-lg">
          {(
            [
              ["downloads", "Downloads"],
              ["volume", "Volume"],
            ] as [Metrica, string][]
          ).map(([chave, rotulo]) => (
            <button
              key={chave}
              type="button"
              onClick={() => setMetrica(chave)}
              className={`px-3 py-1.5 rounded-md text-xs font-medium transition-colors ${
                metrica === chave
                  ? "bg-gray-700 text-white"
                  : "text-gray-400 hover:text-gray-200"
              }`}
            >
              {rotulo}
            </button>
          ))}
        </div>
      </header>

      {series.length === 0 ? (
        <p className="py-16 text-center text-sm text-gray-500">
          Nenhum download no período selecionado.
        </p>
      ) : (
        <div className="relative">
          <svg
            ref={svgRef}
            viewBox={`0 0 ${LARGURA} ${ALTURA}`}
            // Sem altura fixa: com a largura fluida, travar a altura faria o
            // desenho encolher e sobrar faixa vazia dos dois lados do card.
            className="w-full h-auto"
            onMouseMove={aoMover}
            onMouseLeave={() => setAtivo(null)}
            role="img"
            aria-label={
              metrica === "downloads"
                ? "Downloads concluídos e com falha por período"
                : "Volume transferido por período"
            }
          >
            <g transform={`translate(${MARGEM.esquerda} ${MARGEM.topo})`}>
              {/* Grade recessiva: orienta sem competir com as barras. */}
              {marcas.map((marca) => (
                <g key={marca}>
                  <line
                    x1={0}
                    x2={larguraPlot}
                    y1={escalaY(marca)}
                    y2={escalaY(marca)}
                    stroke="#374151"
                    strokeWidth={1}
                  />
                  <text
                    x={-8}
                    y={escalaY(marca) + 4}
                    textAnchor="end"
                    className="fill-gray-500"
                    style={{ fontSize: 10 }}
                  >
                    {rotuloValor(marca)}
                  </text>
                </g>
              ))}

              {series.map((ponto, indice) => {
                const centro = indice * larguraFaixa + larguraFaixa / 2;
                const x = centro - larguraBarra / 2;
                const destaque = ativo === indice;

                if (metrica === "volume") {
                  const altura = alturaPlot - escalaY(ponto.transferred_bytes);
                  return (
                    <rect
                      key={ponto.bucket}
                      x={x}
                      y={escalaY(ponto.transferred_bytes)}
                      width={larguraBarra}
                      height={Math.max(altura, ponto.transferred_bytes > 0 ? 2 : 0)}
                      rx={4}
                      fill="#199e70"
                      opacity={ativo === null || destaque ? 1 : 0.45}
                    />
                  );
                }

                const alturaFalha = alturaPlot - escalaY(ponto.failed);
                const alturaConcluido = alturaPlot - escalaY(ponto.completed);
                // Falhas ficam na base e concluídos empilham por cima, com 2px
                // de respiro entre os dois: segmentos colados leem como um só.
                const baseConcluido = alturaPlot - alturaFalha - (alturaFalha > 0 ? 2 : 0);

                return (
                  <g
                    key={ponto.bucket}
                    opacity={ativo === null || destaque ? 1 : 0.45}
                  >
                    {ponto.failed > 0 && (
                      <rect
                        x={x}
                        y={alturaPlot - alturaFalha}
                        width={larguraBarra}
                        height={Math.max(alturaFalha, 2)}
                        rx={4}
                        fill={COR_FALHA}
                      />
                    )}
                    {ponto.completed > 0 && (
                      <rect
                        x={x}
                        y={baseConcluido - alturaConcluido}
                        width={larguraBarra}
                        height={Math.max(alturaConcluido, 2)}
                        rx={4}
                        fill={COR_CONCLUIDO}
                      />
                    )}
                  </g>
                );
              })}

              <line
                x1={0}
                x2={larguraPlot}
                y1={alturaPlot}
                y2={alturaPlot}
                stroke="#4b5563"
                strokeWidth={1}
              />

              {/* Rótulos do eixo X são amostrados: com 90 pontos, todos
                  colidiriam e nenhum seria legível. */}
              {series.map((ponto, indice) => {
                const passo = Math.ceil(series.length / 8);
                if (indice % passo !== 0) return null;
                return (
                  <text
                    key={ponto.bucket}
                    x={indice * larguraFaixa + larguraFaixa / 2}
                    y={alturaPlot + 16}
                    textAnchor="middle"
                    className="fill-gray-500"
                    style={{ fontSize: 10 }}
                  >
                    {rotuloBucket(ponto.bucket, granularity)}
                  </text>
                );
              })}
            </g>
          </svg>

          {pontoAtivo && (
            <div
              className="pointer-events-none absolute top-2 bg-gray-950 border border-gray-700 rounded-lg px-3 py-2 text-xs shadow-xl"
              style={{
                left: `${Math.min(
                  Math.max(
                    ((MARGEM.esquerda + (ativo! + 0.5) * larguraFaixa) / LARGURA) * 100,
                    8
                  ),
                  78
                )}%`,
              }}
            >
              <p className="text-gray-300 font-medium mb-1">
                {rotuloBucket(pontoAtivo.bucket, granularity)}
              </p>
              {metrica === "downloads" ? (
                <>
                  <p className="flex items-center gap-2 text-gray-400">
                    <span
                      className="w-2 h-2 rounded-sm"
                      style={{ background: COR_CONCLUIDO }}
                    />
                    Concluídos
                    <span className="text-gray-100 ml-auto">
                      {formatNumber(pontoAtivo.completed)}
                    </span>
                  </p>
                  <p className="flex items-center gap-2 text-gray-400 mt-0.5">
                    <span
                      className="w-2 h-2 rounded-sm"
                      style={{ background: COR_FALHA }}
                    />
                    Falhas
                    <span className="text-gray-100 ml-auto">
                      {formatNumber(pontoAtivo.failed)}
                    </span>
                  </p>
                </>
              ) : (
                <p className="text-gray-100">
                  {formatBytes(pontoAtivo.transferred_bytes)}
                </p>
              )}
            </div>
          )}
        </div>
      )}
    </section>
  );
};
