import React, { useState } from "react";
import { Link, useParams } from "react-router-dom";
import {
  Activity,
  AlertTriangle,
  ArrowLeft,
  Download,
  Gauge,
  KeyRound,
  Loader,
  Pencil,
  Send,
  Webhook as WebhookIcon,
} from "lucide-react";
import { AdminShell } from "./AdminShell";
import { StatCard } from "./StatCard";
import { IntegrationFormModal } from "./IntegrationFormModal";
import { IntegrationKeysTab } from "./IntegrationKeysTab";
import { IntegrationWebhooksTab } from "./IntegrationWebhooksTab";
import {
  IntegrationDeliveriesTab,
  IntegrationRequestsTab,
} from "./IntegrationLogsTab";
import { IntegrationDownloadsTab } from "./IntegrationDownloadsTab";
import { useIntegration } from "../../hooks/useIntegrations";
import { formatBytes, formatDateTime, formatNumber } from "./format";

type Aba = "visao" | "chaves" | "webhooks" | "entregas" | "requisicoes" | "downloads";

const ABAS: { id: Aba; rotulo: string; icone: React.ElementType }[] = [
  { id: "visao", rotulo: "Visão geral", icone: Gauge },
  { id: "chaves", rotulo: "Chaves de API", icone: KeyRound },
  { id: "webhooks", rotulo: "Webhooks", icone: WebhookIcon },
  { id: "entregas", rotulo: "Entregas", icone: Send },
  { id: "requisicoes", rotulo: "Requisições e IPs", icone: Activity },
  { id: "downloads", rotulo: "Downloads", icone: Download },
];

const JANELAS = [
  { valor: 24, rotulo: "24 horas" },
  { valor: 24 * 7, rotulo: "7 dias" },
  { valor: 24 * 30, rotulo: "30 dias" },
];

export const IntegrationDetailPage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const [aba, setAba] = useState<Aba>("visao");
  const [janela, setJanela] = useState(24);
  const [editando, setEditando] = useState(false);

  const { data, isLoading, isError } = useIntegration(id, janela);

  if (isLoading) {
    return (
      <AdminShell titulo="Integração">
        <div className="flex justify-center py-16">
          <Loader className="animate-spin text-gray-500" />
        </div>
      </AdminShell>
    );
  }

  if (isError || !data) {
    return (
      <AdminShell titulo="Integração">
        <p className="rounded-xl border border-red-800 bg-red-900/20 p-4 text-sm text-red-200">
          Não foi possível carregar esta integração.
        </p>
        <Link
          to="/admin/integrations"
          className="mt-4 inline-flex items-center gap-2 text-sm text-gray-400 hover:text-gray-200"
        >
          <ArrowLeft size={16} />
          Voltar para a lista
        </Link>
      </AdminShell>
    );
  }

  const { integration, downloads, quota, requests, deliveries } = data;

  return (
    <AdminShell
      titulo={integration.name}
      descricao={integration.description || "Integração de sistema"}
      acoes={
        <>
          <Link
            to="/admin/integrations"
            className="flex items-center gap-2 rounded-lg border border-gray-700 px-4 py-2 text-sm text-gray-300 transition-colors hover:bg-gray-800"
          >
            <ArrowLeft size={16} />
            Integrações
          </Link>
          <button
            type="button"
            onClick={() => setEditando(true)}
            className="flex items-center gap-2 rounded-lg bg-red-600 px-4 py-2 font-medium text-white transition-colors hover:bg-red-700"
          >
            <Pencil size={16} />
            Editar
          </button>
        </>
      }
    >
      {!integration.active && (
        <p className="mb-4 flex items-center gap-2 rounded-xl border border-amber-700/60 bg-amber-950/30 p-3 text-sm text-amber-200">
          <AlertTriangle size={16} className="shrink-0" />
          Esta integração está desativada: toda chamada dela é recusada com{" "}
          <code className="font-mono text-xs">integration_disabled</code>.
        </p>
      )}

      <div className="mb-5 flex flex-wrap items-center gap-2 border-b border-gray-800 pb-3">
        {ABAS.map(({ id: abaId, rotulo, icone: Icone }) => (
          <button
            key={abaId}
            type="button"
            onClick={() => setAba(abaId)}
            className={`flex items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium transition-colors ${
              aba === abaId
                ? "bg-gray-800 text-white"
                : "text-gray-400 hover:bg-gray-800/60 hover:text-gray-200"
            }`}
          >
            <Icone size={15} />
            {rotulo}
          </button>
        ))}

        {(aba === "visao" || aba === "requisicoes") && (
          <select
            value={janela}
            onChange={(evento) => setJanela(Number(evento.target.value))}
            className="ml-auto rounded-lg border border-gray-700 bg-gray-800 px-3 py-2 text-sm text-gray-200"
          >
            {JANELAS.map((opcao) => (
              <option key={opcao.valor} value={opcao.valor}>
                Últimas {opcao.rotulo}
              </option>
            ))}
          </select>
        )}
      </div>

      {aba === "visao" && (
        <div className="space-y-6">
          <section>
            <h3 className="mb-3 text-xs font-semibold uppercase tracking-wide text-gray-500">
              Downloads
            </h3>
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
              <StatCard
                rotulo="Hoje"
                valor={formatNumber(downloads.today)}
                detalhe={
                  quota.daily_limit > 0
                    ? `de ${formatNumber(quota.daily_limit)} · reseta ${formatDateTime(quota.resets_at)}`
                    : "sem teto diário"
                }
                destaque
              />
              <StatCard
                rotulo="Em andamento"
                valor={formatNumber(downloads.active)}
                detalhe={`teto de ${integration.max_concurrent_downloads || "∞"} simultâneos`}
              />
              <StatCard
                rotulo="Concluídos"
                valor={formatNumber(downloads.completed)}
                detalhe={`${formatNumber(downloads.failed)} falharam · ${formatNumber(downloads.canceled)} cancelados`}
              />
              <StatCard
                rotulo="Armazenado"
                valor={formatBytes(downloads.storage_bytes)}
                detalhe={`${formatBytes(downloads.transferred_bytes)} já transferidos`}
              />
            </div>
          </section>

          <section>
            <h3 className="mb-3 text-xs font-semibold uppercase tracking-wide text-gray-500">
              Chamadas à API · últimas {requests.window_hours}h
            </h3>
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
              <StatCard
                rotulo="Requisições"
                valor={formatNumber(requests.total)}
                detalhe={`${formatNumber(requests.unique_ips)} IP(s) distinto(s)`}
              />
              <StatCard
                rotulo="Recusadas"
                valor={formatNumber(requests.client_errors)}
                detalhe={`${formatNumber(requests.throttled)} por limite de chamadas`}
              />
              <StatCard
                rotulo="Erro nosso"
                valor={formatNumber(requests.server_errors)}
                detalhe="respostas 5xx"
              />
              <StatCard
                rotulo="Tempo médio"
                valor={`${formatNumber(requests.avg_duration_ms)} ms`}
                detalhe={`pico de ${formatNumber(requests.max_duration_ms)} ms`}
              />
            </div>
          </section>

          <section>
            <h3 className="mb-3 text-xs font-semibold uppercase tracking-wide text-gray-500">
              Notificações · últimas {deliveries.window_hours}h
            </h3>
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
              <StatCard rotulo="Enviadas" valor={formatNumber(deliveries.total)} />
              <StatCard
                rotulo="Entregues"
                valor={formatNumber(deliveries.delivered)}
                detalhe={`${formatNumber(deliveries.avg_duration_ms)} ms em média`}
              />
              <StatCard
                rotulo="Falhadas"
                valor={formatNumber(deliveries.failed)}
                detalhe="após esgotar as tentativas"
              />
              <StatCard
                rotulo="Pendentes"
                valor={formatNumber(deliveries.pending)}
                detalhe="aguardando entrega ou reenvio"
              />
            </div>
          </section>

          <section className="rounded-xl border border-gray-700 bg-gray-800/60 p-4">
            <h3 className="text-sm font-semibold text-gray-200">Configuração</h3>
            <dl className="mt-3 grid gap-4 text-sm sm:grid-cols-2 lg:grid-cols-3">
              <div>
                <dt className="text-xs text-gray-500">Identificador público</dt>
                <dd className="mt-0.5 break-all font-mono text-xs text-gray-200">
                  {integration.id}
                </dd>
                <p className="mt-1 text-xs text-gray-600">
                  Pode ir no header X-Integration-Id.
                </p>
              </div>
              <div>
                <dt className="text-xs text-gray-500">Chamadas por minuto</dt>
                <dd className="mt-0.5 text-gray-200">
                  {formatNumber(integration.rate_limit_per_minute)}
                </dd>
              </div>
              <div>
                <dt className="text-xs text-gray-500">Tamanho máximo por arquivo</dt>
                <dd className="mt-0.5 text-gray-200">
                  {integration.max_file_size_bytes > 0
                    ? formatBytes(integration.max_file_size_bytes)
                    : "sem teto"}
                </dd>
              </div>
              <div>
                <dt className="text-xs text-gray-500">URL base do sistema</dt>
                <dd className="mt-0.5 break-all text-gray-200">
                  {integration.callback_base_url || "—"}
                </dd>
              </div>
              <div>
                <dt className="text-xs text-gray-500">Conta de serviço</dt>
                <dd className="mt-0.5 break-all font-mono text-xs text-gray-300">
                  {integration.service_email || "—"}
                </dd>
                <p className="mt-1 text-xs text-gray-600">
                  Não faz login: autentica só por chave de API.
                </p>
              </div>
              <div>
                <dt className="text-xs text-gray-500">IPs autorizados</dt>
                <dd className="mt-0.5">
                  {integration.allowed_ips.length === 0 ? (
                    <span className="text-amber-400">qualquer origem</span>
                  ) : (
                    <ul className="space-y-0.5 font-mono text-xs text-gray-200">
                      {integration.allowed_ips.map((ip) => (
                        <li key={ip}>{ip}</li>
                      ))}
                    </ul>
                  )}
                </dd>
              </div>
            </dl>
          </section>
        </div>
      )}

      {aba === "chaves" && id && <IntegrationKeysTab integrationId={id} />}
      {aba === "webhooks" && id && <IntegrationWebhooksTab integrationId={id} />}
      {aba === "entregas" && id && (
        <IntegrationDeliveriesTab integrationId={id} windowHours={janela} />
      )}
      {aba === "requisicoes" && id && (
        <IntegrationRequestsTab integrationId={id} windowHours={janela} />
      )}
      {aba === "downloads" && id && <IntegrationDownloadsTab integrationId={id} />}

      <IntegrationFormModal
        open={editando}
        integracao={integration}
        onClose={() => setEditando(false)}
      />
    </AdminShell>
  );
};
