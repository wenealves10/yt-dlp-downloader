import React, { useState } from "react";
import {
  AlertTriangle,
  CheckCircle,
  Database,
  Download,
  HardDrive,
  Loader,
  Trash2,
  Users,
} from "lucide-react";
import { AdminShell } from "./AdminShell";
import { StatCard } from "./StatCard";
import { DownloadsChart } from "./charts/DownloadsChart";
import { useOverview } from "../../hooks/useAdmin";
import { formatBytes, formatNumber } from "./format";
import type { RangeKey } from "../../interface/Admin";

const ATALHOS: { chave: RangeKey; rotulo: string }[] = [
  { chave: "today", rotulo: "Hoje" },
  { chave: "yesterday", rotulo: "Ontem" },
  { chave: "7d", rotulo: "7 dias" },
  { chave: "15d", rotulo: "15 dias" },
  { chave: "30d", rotulo: "30 dias" },
  { chave: "90d", rotulo: "90 dias" },
  { chave: "12m", rotulo: "12 meses" },
  { chave: "custom", rotulo: "Personalizado" },
];

function hojeISO(): string {
  return new Date().toISOString().slice(0, 10);
}

export const DashboardPage: React.FC = () => {
  const [range, setRange] = useState<RangeKey>("30d");
  const [de, setDe] = useState(hojeISO());
  const [ate, setAte] = useState(hojeISO());

  // No modo personalizado o backend ignora `range`; fora dele as datas não são
  // enviadas, para não fixar o intervalo sem querer.
  const filtros =
    range === "custom" ? { from: de, to: ate } : { range };

  const { data, isLoading, isError, error } = useOverview(filtros);

  const taxaSucesso =
    data && data.downloads.total > 0
      ? Math.round((data.downloads.completed / data.downloads.total) * 100)
      : null;

  return (
    <AdminShell
      titulo="Visão geral"
      descricao="Downloads, usuários e armazenamento da plataforma."
    >
      <div className="mb-6 flex flex-wrap items-center gap-2">
        <div className="flex flex-wrap items-stretch gap-1 p-1 bg-gray-800 rounded-lg">
          {ATALHOS.map(({ chave, rotulo }) => (
            <button
              key={chave}
              type="button"
              onClick={() => setRange(chave)}
              className={`px-3 py-1.5 rounded-md text-xs font-medium transition-colors ${
                range === chave
                  ? "bg-red-600 text-white"
                  : "text-gray-400 hover:text-gray-200"
              }`}
            >
              {rotulo}
            </button>
          ))}
        </div>

        {range === "custom" && (
          <div className="flex items-center gap-2 text-xs text-gray-400">
            <input
              type="date"
              value={de}
              max={ate}
              onChange={(evento) => setDe(evento.target.value)}
              className="bg-gray-800 border border-gray-700 rounded-lg px-3 py-1.5 text-gray-200"
            />
            <span>até</span>
            <input
              type="date"
              value={ate}
              min={de}
              max={hojeISO()}
              onChange={(evento) => setAte(evento.target.value)}
              className="bg-gray-800 border border-gray-700 rounded-lg px-3 py-1.5 text-gray-200"
            />
          </div>
        )}
      </div>

      {isLoading && (
        <p className="flex items-center gap-2 text-gray-400">
          <Loader className="animate-spin" size={18} /> Carregando métricas...
        </p>
      )}

      {isError && (
        <div className="flex items-start gap-3 bg-red-900/30 border border-red-700 text-red-200 rounded-lg p-4 text-sm">
          <AlertTriangle size={18} className="mt-0.5 shrink-0" />
          <span>{error instanceof Error ? error.message : "Falha ao carregar."}</span>
        </div>
      )}

      {data && (
        <div className="space-y-6">
          <section className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <StatCard
              destaque
              rotulo="Downloads no período"
              valor={formatNumber(data.downloads.total)}
              detalhe={
                taxaSucesso === null
                  ? "Sem downloads no período"
                  : `${taxaSucesso}% concluídos`
              }
              icone={Download}
            />
            <StatCard
              rotulo="Concluídos"
              valor={formatNumber(data.downloads.completed)}
              detalhe={`${formatNumber(data.downloads.processing)} em andamento`}
              icone={CheckCircle}
            />
            <StatCard
              rotulo="Falhas"
              valor={formatNumber(data.downloads.failed)}
              detalhe={`${formatNumber(data.downloads.expired)} expirados`}
              icone={AlertTriangle}
            />
            <StatCard
              rotulo="Usuários ativos"
              valor={formatNumber(data.downloads.active_users)}
              detalhe="baixaram algo no período"
              icone={Users}
            />
          </section>

          <DownloadsChart
            series={data.series}
            granularity={data.period.granularity}
          />

          <section className="grid gap-4 lg:grid-cols-3">
            <div className="lg:col-span-2 grid gap-4 sm:grid-cols-2">
              <StatCard
                destaque
                rotulo="Armazenado agora"
                valor={formatBytes(data.storage.stored_bytes)}
                detalhe={`${formatNumber(data.storage.stored_files)} arquivos no bucket`}
                icone={HardDrive}
              />
              <StatCard
                rotulo="Já liberado"
                valor={formatBytes(data.storage.freed_bytes)}
                detalhe={`${formatNumber(data.storage.freed_files)} arquivos expirados ou removidos`}
                icone={Trash2}
              />
              <StatCard
                rotulo="Transferido no período"
                valor={formatBytes(data.downloads.transferred_bytes)}
                detalhe="soma dos arquivos entregues"
                icone={Download}
              />
              <StatCard
                rotulo="Histórico total"
                valor={formatBytes(data.storage.lifetime_bytes)}
                detalhe={`${formatNumber(data.platform.downloads_total)} downloads desde o início`}
                icone={Database}
              />
            </div>

            <div className="bg-gray-800 border border-gray-700 rounded-xl p-5">
              <h2 className="text-base font-semibold text-gray-100 mb-4">
                Plataforma
              </h2>
              <dl className="space-y-3 text-sm">
                {[
                  ["Usuários cadastrados", formatNumber(data.platform.users_total)],
                  ["Ativos", formatNumber(data.platform.users_active)],
                  ["Premium", formatNumber(data.platform.users_premium)],
                  ["Novos em 30 dias", formatNumber(data.platform.users_new_30d)],
                ].map(([rotulo, valor]) => (
                  <div key={rotulo} className="flex items-center justify-between gap-4">
                    <dt className="text-gray-400">{rotulo}</dt>
                    <dd className="text-gray-100 font-medium tabular-nums">{valor}</dd>
                  </div>
                ))}
              </dl>

              {data.formats && data.formats.length > 0 && (
                <>
                  <h3 className="text-xs font-medium text-gray-400 uppercase tracking-wide mt-6 mb-3">
                    Por formato
                  </h3>
                  <dl className="space-y-2 text-sm">
                    {data.formats.map((formato) => (
                      <div
                        key={formato.format}
                        className="flex items-center justify-between gap-4"
                      >
                        <dt className="text-gray-400">{formato.format}</dt>
                        <dd className="text-gray-100 tabular-nums">
                          {formatNumber(formato.total)}
                          <span className="text-gray-500 ml-2">
                            {formatBytes(formato.transferred_bytes)}
                          </span>
                        </dd>
                      </div>
                    ))}
                  </dl>
                </>
              )}
            </div>
          </section>

          {data.top_users && data.top_users.length > 0 && (
            <section className="bg-gray-800 border border-gray-700 rounded-xl p-5">
              <h2 className="text-base font-semibold text-gray-100 mb-4">
                Quem mais baixou no período
              </h2>
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="text-left text-xs text-gray-500 uppercase tracking-wide">
                      <th className="pb-2 font-medium">Usuário</th>
                      <th className="pb-2 font-medium">Plano</th>
                      <th className="pb-2 font-medium text-right">Downloads</th>
                      <th className="pb-2 font-medium text-right">Volume</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-700">
                    {data.top_users.map((usuario) => (
                      <tr key={usuario.id}>
                        <td className="py-2.5 pr-4">
                          <p className="text-gray-100">{usuario.full_name}</p>
                          <p className="text-xs text-gray-500">{usuario.email}</p>
                        </td>
                        <td className="py-2.5 pr-4 text-gray-400">{usuario.plan}</td>
                        <td className="py-2.5 text-right text-gray-100 tabular-nums">
                          {formatNumber(usuario.downloads_total)}
                        </td>
                        <td className="py-2.5 text-right text-gray-400 tabular-nums">
                          {formatBytes(usuario.transferred_bytes)}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </section>
          )}
        </div>
      )}
    </AdminShell>
  );
};
