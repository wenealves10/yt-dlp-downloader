import React, { useState } from "react";
import {
  Loader,
  Plus,
  Power,
  RefreshCw,
  Send,
  Trash2,
  TriangleAlert,
  Webhook as WebhookIcon,
} from "lucide-react";
import {
  useCreateWebhook,
  useDeleteWebhook,
  useTestWebhook,
  useUpdateWebhook,
  useWebhooks,
} from "../../hooks/useIntegrations";
import { formatDateTime } from "./format";
import { SegredoRevelado } from "./SegredoRevelado";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import { EVENTOS_WEBHOOK, type Webhook } from "../../interface/Integration";

interface Props {
  integrationId: string;
}

export const IntegrationWebhooksTab: React.FC<Props> = ({ integrationId }) => {
  const { data, isLoading } = useWebhooks(integrationId);
  const criar = useCreateWebhook();
  const atualizar = useUpdateWebhook();
  const remover = useDeleteWebhook();
  const testar = useTestWebhook();

  const [url, setUrl] = useState("");
  const [eventos, setEventos] = useState<string[]>([]);
  const [comProgresso, setComProgresso] = useState(false);
  const [erro, setErro] = useState("");
  const [aviso, setAviso] = useState("");
  const [removendo, setRemovendo] = useState<Webhook | null>(null);
  const [rotacionando, setRotacionando] = useState<Webhook | null>(null);

  const alternarEvento = (evento: string) => {
    setEventos((atual) =>
      atual.includes(evento)
        ? atual.filter((item) => item !== evento)
        : [...atual, evento]
    );
  };

  const cadastrar = (submit: React.FormEvent) => {
    submit.preventDefault();
    setErro("");
    setAviso("");
    criar.mutate(
      {
        id: integrationId,
        url: url.trim(),
        // Lista vazia significa "todos os de ciclo de vida", que é o padrão
        // desejável: um webhook que não assina nada nasce mudo.
        events: eventos,
        include_progress: comProgresso,
      },
      {
        onSuccess: () => {
          setUrl("");
          setEventos([]);
          setComProgresso(false);
          setAviso("Webhook cadastrado. Use “Testar” para validar o endpoint.");
        },
        onError: (falha) => setErro(falha.message),
      }
    );
  };

  const webhooks = data?.webhooks ?? [];

  return (
    <div className="space-y-5">
      <section className="rounded-xl border border-gray-700 bg-gray-800/60 p-4">
        <h3 className="text-sm font-semibold text-gray-200">Cadastrar webhook</h3>
        <p className="mt-1 text-xs text-gray-500">
          Assim que o download muda de estado, fazemos um POST assinado nesta
          URL. É o que dispensa o sistema integrado de ficar consultando a rota
          de downloads em laço.
        </p>

        <form onSubmit={cadastrar} className="mt-3 space-y-3">
          <label className="block">
            <span className="text-xs text-gray-400">URL de destino</span>
            <input
              required
              value={url}
              onChange={(evento) => setUrl(evento.target.value)}
              placeholder="https://crm.suaempresa.com/webhooks/advideo"
              className="mt-1 w-full rounded-lg border border-gray-700 bg-gray-900 px-3 py-2 text-sm text-gray-100 placeholder-gray-600"
            />
            <span className="mt-1 block text-xs text-gray-500">
              Precisa ser https e apontar para um endereço público — endereços
              internos são recusados de propósito.
            </span>
          </label>

          <div>
            <span className="text-xs text-gray-400">Eventos</span>
            <div className="mt-1.5 flex flex-wrap gap-1.5">
              {EVENTOS_WEBHOOK.map((evento) => {
                const marcado = eventos.includes(evento);
                return (
                  <button
                    key={evento}
                    type="button"
                    onClick={() => alternarEvento(evento)}
                    className={`rounded-lg border px-2.5 py-1 font-mono text-xs transition-colors ${
                      marcado
                        ? "border-red-600 bg-red-600/20 text-red-200"
                        : "border-gray-700 text-gray-400 hover:bg-gray-700"
                    }`}
                  >
                    {evento}
                  </button>
                );
              })}
            </div>
            <span className="mt-1.5 block text-xs text-gray-500">
              Nenhum selecionado = todos os de ciclo de vida.
            </span>
          </div>

          <label className="flex items-start gap-2 text-sm text-gray-300">
            <input
              type="checkbox"
              checked={comProgresso}
              onChange={(evento) => setComProgresso(evento.target.checked)}
              className="mt-0.5 h-4 w-4 rounded border-gray-600 bg-gray-900 text-red-600"
            />
            <span>
              Enviar também o progresso
              <span className="block text-xs text-gray-500">
                Muitos POSTs por download (espaçados em alguns segundos). Só
                vale se o outro sistema mostra barra de progresso.
              </span>
            </span>
          </label>

          {erro && (
            <p className="rounded-lg border border-red-700 bg-red-900/30 p-3 text-sm text-red-200">
              {erro}
            </p>
          )}
          {aviso && (
            <p className="rounded-lg border border-green-800 bg-green-900/20 p-3 text-sm text-green-200">
              {aviso}
            </p>
          )}

          <button
            type="submit"
            disabled={criar.isPending}
            className="flex items-center gap-2 rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-red-700 disabled:opacity-60"
          >
            {criar.isPending ? (
              <Loader size={16} className="animate-spin" />
            ) : (
              <Plus size={16} />
            )}
            Cadastrar
          </button>
        </form>
      </section>

      {isLoading ? (
        <div className="flex justify-center py-10">
          <Loader className="animate-spin text-gray-500" />
        </div>
      ) : webhooks.length === 0 ? (
        <p className="rounded-xl border border-gray-700 bg-gray-800/40 p-6 text-center text-sm text-gray-400">
          Nenhum webhook. Sem ele, o sistema integrado precisa consultar{" "}
          <code className="text-gray-500">GET /v1/integration/downloads</code>{" "}
          para saber quando um download terminou.
        </p>
      ) : (
        <div className="space-y-3">
          {webhooks.map((webhook) => (
            <article
              key={webhook.id}
              className="rounded-xl border border-gray-700 bg-gray-800/60 p-4"
            >
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div className="min-w-0">
                  <p className="flex items-center gap-2 break-all font-mono text-sm text-gray-100">
                    <WebhookIcon size={16} className="shrink-0 text-gray-500" />
                    {webhook.url}
                  </p>
                  <div className="mt-2 flex flex-wrap gap-1">
                    {webhook.events.map((evento) => (
                      <span
                        key={evento}
                        className="rounded bg-gray-700/70 px-1.5 py-0.5 font-mono text-[11px] text-gray-300"
                      >
                        {evento}
                      </span>
                    ))}
                    {webhook.include_progress && (
                      <span className="rounded bg-blue-600/20 px-1.5 py-0.5 font-mono text-[11px] text-blue-300">
                        download.progress
                      </span>
                    )}
                  </div>
                </div>

                <div className="flex shrink-0 items-center gap-1">
                  <button
                    type="button"
                    onClick={() =>
                      testar.mutate(
                        { id: integrationId, webhookId: webhook.id },
                        {
                          onSuccess: () =>
                            setAviso(
                              "Evento de teste enfileirado. O resultado aparece na aba Entregas."
                            ),
                          onError: (falha) => setErro(falha.message),
                        }
                      )
                    }
                    disabled={testar.isPending}
                    className="flex items-center gap-1.5 rounded-lg border border-gray-600 px-3 py-1.5 text-xs text-gray-300 transition-colors hover:bg-gray-700 disabled:opacity-50"
                    title="Envia um evento ping para este endpoint"
                  >
                    <Send size={14} />
                    Testar
                  </button>
                  <button
                    type="button"
                    onClick={() =>
                      atualizar.mutate({
                        id: integrationId,
                        webhookId: webhook.id,
                        active: !webhook.active,
                      })
                    }
                    className="rounded-lg p-2 text-gray-400 transition-colors hover:bg-gray-700 hover:text-gray-200"
                    title={webhook.active ? "Desativar" : "Reativar"}
                    aria-label={webhook.active ? "Desativar" : "Reativar"}
                  >
                    <Power size={16} className={webhook.active ? "text-green-400" : ""} />
                  </button>
                  <button
                    type="button"
                    onClick={() => setRotacionando(webhook)}
                    className="rounded-lg p-2 text-gray-400 transition-colors hover:bg-gray-700 hover:text-gray-200"
                    title="Rotacionar segredo de assinatura"
                    aria-label="Rotacionar segredo"
                  >
                    <RefreshCw size={16} />
                  </button>
                  <button
                    type="button"
                    onClick={() => setRemovendo(webhook)}
                    className="rounded-lg p-2 text-gray-400 transition-colors hover:bg-red-900/40 hover:text-red-300"
                    title="Remover"
                    aria-label="Remover"
                  >
                    <Trash2 size={16} />
                  </button>
                </div>
              </div>

              <div className="mt-3 grid gap-3 border-t border-gray-700/70 pt-3 text-xs sm:grid-cols-3">
                <div>
                  <p className="text-gray-500">Última entrega</p>
                  <p className="mt-0.5 text-gray-300">
                    {formatDateTime(webhook.last_delivery_at)}
                    {!!webhook.last_status_code && (
                      <span
                        className={
                          webhook.last_status_code < 300
                            ? " text-green-400"
                            : " text-red-400"
                        }
                      >
                        {" "}
                        · HTTP {webhook.last_status_code}
                      </span>
                    )}
                  </p>
                </div>
                <div>
                  <p className="text-gray-500">Falhas seguidas</p>
                  <p
                    className={`mt-0.5 ${
                      webhook.consecutive_failures > 0 ? "text-amber-300" : "text-gray-300"
                    }`}
                  >
                    {webhook.consecutive_failures}
                  </p>
                </div>
                <div>
                  <p className="text-gray-500">Situação</p>
                  <p className="mt-0.5">
                    {webhook.active ? (
                      <span className="text-green-400">ativo</span>
                    ) : (
                      <span className="text-gray-400">desativado</span>
                    )}
                  </p>
                </div>
              </div>

              {webhook.disabled_reason && (
                <p className="mt-3 flex items-start gap-1.5 rounded-lg border border-amber-700/50 bg-amber-950/30 p-2.5 text-xs text-amber-200">
                  <TriangleAlert size={14} className="mt-0.5 shrink-0" />
                  {webhook.disabled_reason}
                </p>
              )}

              {webhook.last_error && (
                <p className="mt-2 break-words rounded-lg bg-gray-950/50 p-2.5 font-mono text-[11px] text-gray-400">
                  {webhook.last_error}
                </p>
              )}

              {webhook.secret && (
                <div className="mt-3">
                  <SegredoRevelado
                    titulo="Segredo de assinatura"
                    valor={webhook.secret}
                    descricao="O sistema integrado usa este valor para verificar o header X-Advideo-Signature. Nunca sai pela API — só aqui."
                  />
                </div>
              )}
            </article>
          ))}
        </div>
      )}

      <ConfirmDialog
        open={!!removendo}
        titulo="Remover este webhook?"
        alvo={removendo?.url}
        descricao="As notificações param imediatamente. O histórico de entregas é preservado."
        confirmarLabel="Sim, remover"
        processando={remover.isPending}
        onConfirm={() =>
          removendo &&
          remover.mutate(
            { id: integrationId, webhookId: removendo.id },
            { onSuccess: () => setRemovendo(null) }
          )
        }
        onCancel={() => setRemovendo(null)}
      />

      <ConfirmDialog
        open={!!rotacionando}
        titulo="Rotacionar o segredo de assinatura?"
        alvo={rotacionando?.url}
        descricao="O segredo atual deixa de validar imediatamente. O sistema integrado vai recusar as notificações até ser atualizado com o novo valor."
        confirmarLabel="Sim, rotacionar"
        processando={atualizar.isPending}
        onConfirm={() =>
          rotacionando &&
          atualizar.mutate(
            {
              id: integrationId,
              webhookId: rotacionando.id,
              rotate_secret: true,
            },
            { onSuccess: () => setRotacionando(null) }
          )
        }
        onCancel={() => setRotacionando(null)}
      />
    </div>
  );
};
