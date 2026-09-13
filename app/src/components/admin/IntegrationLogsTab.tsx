import React, { useState } from "react";
import { ChevronLeft, ChevronRight, Globe, Loader, RotateCw, ShieldAlert } from "lucide-react";
import {
  useDeliveries,
  useIntegrationRequests,
  useIntegrationTraffic,
  useRetryDelivery,
} from "../../hooks/useIntegrations";
import { formatDateTime, formatNumber } from "./format";
import type { Delivery } from "../../interface/Integration";

interface Props {
  integrationId: string;
  windowHours: number;
}

// Cor pela CLASSE do status, não pelo código exato: o operador está varrendo a
// lista procurando o que está vermelho, não lendo cada número.
function corDoStatus(status: number): string {
  if (status >= 500) return "text-red-400";
  if (status === 429) return "text-amber-300";
  if (status >= 400) return "text-orange-300";
  return "text-green-400";
}

export const IntegrationDeliveriesTab: React.FC<Props> = ({ integrationId }) => {
  const [page, setPage] = useState(1);
  const [status, setStatus] = useState("");
  const [expandida, setExpandida] = useState<string | null>(null);
  const { data, isLoading } = useDeliveries(integrationId, { page, perPage: 20, status });
  const reenviar = useRetryDelivery();

  const entregas = data?.deliveries ?? [];

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-2">
        <select
          value={status}
          onChange={(evento) => {
            setPage(1);
            setStatus(evento.target.value);
          }}
          className="rounded-lg border border-gray-700 bg-gray-800 px-3 py-2 text-sm text-gray-200"
        >
          <option value="">Todas as entregas</option>
          <option value="DELIVERED">Entregues</option>
          <option value="FAILED">Falhadas</option>
          <option value="PENDING">Pendentes</option>
        </select>
        <p className="text-xs text-gray-500">
          É aqui que se responde “vocês enviaram?”: o corpo exato do POST, o que
          o endpoint respondeu e quantas tentativas foram feitas.
        </p>
      </div>

      {isLoading ? (
        <div className="flex justify-center py-10">
          <Loader className="animate-spin text-gray-500" />
        </div>
      ) : entregas.length === 0 ? (
        <p className="rounded-xl border border-gray-700 bg-gray-800/40 p-6 text-center text-sm text-gray-400">
          Nenhuma entrega registrada ainda.
        </p>
      ) : (
        <div className="overflow-x-auto rounded-xl border border-gray-700">
          <table className="min-w-full divide-y divide-gray-700 text-sm">
            <thead className="bg-gray-800/80 text-xs uppercase tracking-wide text-gray-400">
              <tr>
                <th className="px-4 py-3 text-left">Quando</th>
                <th className="px-4 py-3 text-left">Evento</th>
                <th className="px-4 py-3 text-left">Situação</th>
                <th className="px-4 py-3 text-left">Tentativas</th>
                <th className="px-4 py-3 text-left">Resposta</th>
                <th className="px-4 py-3 text-right">Ações</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-800 bg-gray-900/40">
              {entregas.map((entrega: Delivery) => (
                <React.Fragment key={entrega.id}>
                  <tr className="hover:bg-gray-800/40">
                    <td className="px-4 py-3 text-gray-400">
                      {formatDateTime(entrega.created_at)}
                    </td>
                    <td className="px-4 py-3 font-mono text-xs text-gray-200">
                      {entrega.event_type}
                    </td>
                    <td className="px-4 py-3">
                      {entrega.status === "DELIVERED" ? (
                        <span className="text-green-400">entregue</span>
                      ) : entrega.status === "FAILED" ? (
                        <span className="text-red-400">falhou</span>
                      ) : (
                        <span className="text-amber-300">pendente</span>
                      )}
                    </td>
                    <td className="px-4 py-3 tabular-nums text-gray-300">
                      {entrega.attempts}
                    </td>
                    <td className="px-4 py-3">
                      {entrega.last_status_code ? (
                        <span className={corDoStatus(entrega.last_status_code)}>
                          HTTP {entrega.last_status_code}
                        </span>
                      ) : (
                        <span className="text-gray-600">—</span>
                      )}
                      {entrega.duration_ms > 0 && (
                        <span className="text-gray-600"> · {entrega.duration_ms}ms</span>
                      )}
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex justify-end gap-1">
                        <button
                          type="button"
                          onClick={() =>
                            setExpandida((atual) =>
                              atual === entrega.id ? null : entrega.id
                            )
                          }
                          className="rounded-lg px-2 py-1 text-xs text-gray-400 transition-colors hover:bg-gray-700 hover:text-gray-200"
                        >
                          {expandida === entrega.id ? "Fechar" : "Ver corpo"}
                        </button>
                        {entrega.status !== "DELIVERED" && (
                          <button
                            type="button"
                            onClick={() =>
                              reenviar.mutate({
                                id: integrationId,
                                deliveryId: entrega.id,
                              })
                            }
                            disabled={reenviar.isPending}
                            className="rounded-lg p-1.5 text-gray-400 transition-colors hover:bg-gray-700 hover:text-gray-200 disabled:opacity-50"
                            title="Reenviar esta entrega"
                            aria-label="Reenviar"
                          >
                            <RotateCw size={14} />
                          </button>
                        )}
                      </div>
                    </td>
                  </tr>

                  {expandida === entrega.id && (
                    <tr className="bg-gray-950/50">
                      <td colSpan={6} className="px-4 py-3">
                        {entrega.last_error && (
                          <p className="mb-2 break-words rounded-lg border border-red-800/60 bg-red-950/30 p-2.5 font-mono text-[11px] text-red-200">
                            {entrega.last_error}
                          </p>
                        )}
                        <p className="mb-1 text-xs text-gray-500">
                          Destino: <span className="font-mono">{entrega.webhook_url}</span>
                        </p>
                        {/* O payload gravado é o que foi enviado de fato — e é
                            ele que uma reentrega repete, não um remontado a
                            partir do estado atual do download. */}
                        <pre className="max-h-64 overflow-auto rounded-lg bg-black/40 p-3 font-mono text-[11px] leading-relaxed text-gray-300">
                          {JSON.stringify(entrega.payload, null, 2)}
                        </pre>
                      </td>
                    </tr>
                  )}
                </React.Fragment>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <Paginacao
        page={data?.page ?? 1}
        total={data?.total ?? 0}
        perPage={data?.per_page ?? 20}
        temAnterior={!!data?.prev_page}
        temProxima={!!data?.next_page}
        onPage={setPage}
        rotulo="entrega(s)"
      />
    </div>
  );
};

// ---------------------------------------------------------------------------
// Requisições e origens
// ---------------------------------------------------------------------------

export const IntegrationRequestsTab: React.FC<Props> = ({
  integrationId,
  windowHours,
}) => {
  const [page, setPage] = useState(1);
  const [classe, setClasse] = useState("");
  const [ip, setIp] = useState("");
  const [ipAplicado, setIpAplicado] = useState("");

  const { data, isLoading } = useIntegrationRequests(integrationId, {
    page,
    perPage: 25,
    status_class: classe,
    ip: ipAplicado,
  });
  const { data: trafego } = useIntegrationTraffic(integrationId, windowHours);

  const requisicoes = data?.requests ?? [];

  return (
    <div className="space-y-5">
      <div className="grid gap-4 lg:grid-cols-2">
        <section className="rounded-xl border border-gray-700 bg-gray-800/60 p-4">
          <h3 className="flex items-center gap-2 text-sm font-semibold text-gray-200">
            <Globe size={16} className="text-gray-500" />
            De onde vêm as chamadas
          </h3>
          <p className="mt-1 text-xs text-gray-500">
            Últimas {windowHours}h. É a resposta para “quem está batendo nesta
            chave” — e o lugar de descobrir um IP que não deveria estar aqui.
          </p>

          {!trafego ? (
            <div className="flex justify-center py-6">
              <Loader size={18} className="animate-spin text-gray-500" />
            </div>
          ) : trafego.top_ips.length === 0 ? (
            <p className="py-4 text-sm text-gray-500">Nenhuma chamada no período.</p>
          ) : (
            <ul className="mt-3 divide-y divide-gray-700/70">
              {trafego.top_ips.map((origem) => (
                <li
                  key={origem.ip}
                  className="flex items-center justify-between gap-3 py-2 text-sm"
                >
                  <button
                    type="button"
                    onClick={() => {
                      setIp(origem.ip);
                      setIpAplicado(origem.ip);
                      setPage(1);
                    }}
                    className="min-w-0 text-left font-mono text-xs text-gray-200 hover:text-red-400"
                    title="Filtrar as requisições deste IP"
                  >
                    {origem.ip}
                  </button>
                  <div className="flex shrink-0 items-center gap-3 text-xs">
                    {!origem.allowed && (
                      <span
                        className="flex items-center gap-1 text-amber-300"
                        title="Este IP não passaria pela lista de IPs autorizados configurada"
                      >
                        <ShieldAlert size={13} />
                        fora da lista
                      </span>
                    )}
                    <span className="tabular-nums text-gray-300">
                      {formatNumber(origem.requests)}
                    </span>
                    {origem.errors > 0 && (
                      <span className="tabular-nums text-red-400">
                        {formatNumber(origem.errors)} erro(s)
                      </span>
                    )}
                  </div>
                </li>
              ))}
            </ul>
          )}
        </section>

        <section className="rounded-xl border border-gray-700 bg-gray-800/60 p-4">
          <h3 className="text-sm font-semibold text-gray-200">Erros por causa</h3>
          <p className="mt-1 text-xs text-gray-500">
            Agrupados pelo código estável, não pelo texto — é assim que se vê o
            que o sistema integrado está errando de forma repetida.
          </p>

          {!trafego ? (
            <div className="flex justify-center py-6">
              <Loader size={18} className="animate-spin text-gray-500" />
            </div>
          ) : trafego.top_errors.length === 0 ? (
            <p className="py-4 text-sm text-gray-500">
              Nenhum erro no período. É o que se espera de uma integração
              saudável.
            </p>
          ) : (
            <ul className="mt-3 divide-y divide-gray-700/70">
              {trafego.top_errors.map((falha) => (
                <li
                  key={`${falha.error_code}-${falha.status_code}`}
                  className="flex items-center justify-between gap-3 py-2 text-sm"
                >
                  <span className="min-w-0 font-mono text-xs text-gray-200">
                    {falha.error_code}
                  </span>
                  <div className="flex shrink-0 items-center gap-3 text-xs">
                    <span className={corDoStatus(falha.status_code)}>
                      HTTP {falha.status_code}
                    </span>
                    <span className="tabular-nums text-gray-300">
                      {formatNumber(falha.occurrences)}×
                    </span>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </section>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <select
          value={classe}
          onChange={(evento) => {
            setPage(1);
            setClasse(evento.target.value);
          }}
          className="rounded-lg border border-gray-700 bg-gray-800 px-3 py-2 text-sm text-gray-200"
        >
          <option value="">Todas as respostas</option>
          <option value="2">Sucesso (2xx)</option>
          <option value="4">Recusadas (4xx)</option>
          <option value="5">Erro nosso (5xx)</option>
        </select>

        <form
          onSubmit={(evento) => {
            evento.preventDefault();
            setPage(1);
            setIpAplicado(ip.trim());
          }}
          className="flex gap-2"
        >
          <input
            value={ip}
            onChange={(evento) => setIp(evento.target.value)}
            placeholder="Filtrar por IP"
            className="w-44 rounded-lg border border-gray-700 bg-gray-800 px-3 py-2 font-mono text-xs text-gray-200 placeholder-gray-500"
          />
          <button
            type="submit"
            className="rounded-lg border border-gray-700 px-3 py-2 text-sm text-gray-300 transition-colors hover:bg-gray-800"
          >
            Filtrar
          </button>
          {ipAplicado && (
            <button
              type="button"
              onClick={() => {
                setIp("");
                setIpAplicado("");
                setPage(1);
              }}
              className="rounded-lg px-3 py-2 text-sm text-gray-400 transition-colors hover:bg-gray-800"
            >
              Limpar
            </button>
          )}
        </form>
      </div>

      {isLoading ? (
        <div className="flex justify-center py-10">
          <Loader className="animate-spin text-gray-500" />
        </div>
      ) : requisicoes.length === 0 ? (
        <p className="rounded-xl border border-gray-700 bg-gray-800/40 p-6 text-center text-sm text-gray-400">
          Nenhuma requisição registrada com estes filtros.
        </p>
      ) : (
        <div className="overflow-x-auto rounded-xl border border-gray-700">
          <table className="min-w-full divide-y divide-gray-700 text-sm">
            <thead className="bg-gray-800/80 text-xs uppercase tracking-wide text-gray-400">
              <tr>
                <th className="px-4 py-3 text-left">Quando</th>
                <th className="px-4 py-3 text-left">Chamada</th>
                <th className="px-4 py-3 text-left">Resposta</th>
                <th className="px-4 py-3 text-left">Causa</th>
                <th className="px-4 py-3 text-left">Origem</th>
                <th className="px-4 py-3 text-left">Tempo</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-800 bg-gray-900/40">
              {requisicoes.map((linha) => (
                <tr key={linha.id} className="hover:bg-gray-800/40">
                  <td className="whitespace-nowrap px-4 py-2.5 text-xs text-gray-400">
                    {formatDateTime(linha.created_at)}
                  </td>
                  <td className="px-4 py-2.5">
                    <span className="font-mono text-xs text-gray-400">{linha.method}</span>{" "}
                    <span className="break-all font-mono text-xs text-gray-200">
                      {linha.path}
                    </span>
                  </td>
                  <td className="px-4 py-2.5">
                    <span className={`font-mono text-xs ${corDoStatus(linha.status_code)}`}>
                      {linha.status_code}
                    </span>
                  </td>
                  <td className="px-4 py-2.5 font-mono text-xs text-gray-400">
                    {linha.error_code || "—"}
                  </td>
                  <td className="px-4 py-2.5 font-mono text-xs text-gray-400">
                    {linha.ip}
                  </td>
                  <td className="px-4 py-2.5 tabular-nums text-xs text-gray-400">
                    {linha.duration_ms}ms
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <Paginacao
        page={data?.page ?? 1}
        total={data?.total ?? 0}
        perPage={data?.per_page ?? 25}
        temAnterior={!!data?.prev_page}
        temProxima={!!data?.next_page}
        onPage={setPage}
        rotulo="requisição(ões)"
      />
    </div>
  );
};

// ---------------------------------------------------------------------------

interface PaginacaoProps {
  page: number;
  total: number;
  perPage: number;
  temAnterior: boolean;
  temProxima: boolean;
  onPage: (valor: number) => void;
  rotulo: string;
}

export const Paginacao: React.FC<PaginacaoProps> = ({
  page,
  total,
  perPage,
  temAnterior,
  temProxima,
  onPage,
  rotulo,
}) => {
  if (total <= perPage) return null;

  return (
    <div className="flex items-center justify-between text-sm text-gray-400">
      <span>
        Página {page} — {formatNumber(total)} {rotulo}
      </span>
      <div className="flex gap-2">
        <button
          type="button"
          disabled={!temAnterior}
          onClick={() => onPage(Math.max(1, page - 1))}
          className="flex items-center gap-1 rounded-lg border border-gray-700 px-3 py-1.5 transition-colors hover:bg-gray-800 disabled:opacity-40"
        >
          <ChevronLeft size={16} />
          Anterior
        </button>
        <button
          type="button"
          disabled={!temProxima}
          onClick={() => onPage(page + 1)}
          className="flex items-center gap-1 rounded-lg border border-gray-700 px-3 py-1.5 transition-colors hover:bg-gray-800 disabled:opacity-40"
        >
          Próxima
          <ChevronRight size={16} />
        </button>
      </div>
    </div>
  );
};
