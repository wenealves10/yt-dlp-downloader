import React from "react";
import { useQuery } from "@tanstack/react-query";
import {
  AlertTriangle,
  CheckCircle,
  Clock,
  Cpu,
  Loader,
  XCircle,
} from "lucide-react";
import { AdminShell } from "./AdminShell";
import { useAuth } from "../../hooks/useAuth";

type Uso = "required" | "optional" | "unused";

interface Ferramenta {
  name: string;
  available: boolean;
  version?: string;
  required: boolean;
  usage?: Uso;
  note?: string;
}

/**
 * "Opcional" e "não usada nesta etapa" são coisas diferentes, e o painel já
 * confundiu as duas: o deno aparecia como não usado quando na verdade é
 * executado nas duas etapas. Um worker de versão anterior publica relatório sem
 * `usage`, então cai no booleano antigo.
 */
function usoDe(ferramenta: Ferramenta): Uso {
  return ferramenta.usage ?? (ferramenta.required ? "required" : "optional");
}

const ROTULO_USO: Record<Uso, string> = {
  required: "obrigatória",
  optional: "opcional",
  unused: "não usada nesta etapa",
};

interface ProviderSaude {
  provider: string;
  role?: string;
  available: boolean;
  version?: string;
  detail?: string;
  tools?: Ferramenta[];
  checked_at: string;
}

/**
 * Uma etapa é um dos processos que executam o provider. A API só resolve
 * metadados; quem baixa e junta as faixas é o worker, em outra imagem e com
 * outras dependências — daí os dois blocos separados na tela.
 */
interface Etapa {
  role: string;
  label: string;
  source: string;
  providers: ProviderSaude[];
  available: boolean;
  detail?: string;
  reported_at?: string;
  stale: boolean;
}

interface Resposta {
  stages: Etapa[];
  healthy: boolean;
  platforms?: { id: string; label: string }[];
}

const ORIGEM: Record<string, string> = {
  api: "container da API",
  worker: "container do worker",
};

function formatarQuando(iso?: string): string | null {
  if (!iso) return null;
  const data = new Date(iso);
  if (Number.isNaN(data.getTime())) return null;

  const segundos = Math.round((Date.now() - data.getTime()) / 1000);
  if (segundos < 60) return "agora há pouco";
  if (segundos < 3600) return `há ${Math.floor(segundos / 60)} min`;
  if (segundos < 86400) return `há ${Math.floor(segundos / 3600)} h`;
  return data.toLocaleString("pt-BR");
}

const TabelaFerramentas: React.FC<{ ferramentas: Ferramenta[] }> = ({
  ferramentas,
}) => (
  <div className="overflow-x-auto">
    <table className="w-full text-sm">
      <thead>
        <tr className="text-left text-xs text-gray-400 uppercase tracking-wide">
          <th className="pb-2 font-medium">Dependência</th>
          <th className="pb-2 font-medium">Estado</th>
          <th className="pb-2 font-medium">Versão</th>
        </tr>
      </thead>
      <tbody className="divide-y divide-gray-700">
        {ferramentas.map((ferramenta) => {
          const uso = usoDe(ferramenta);
          return (
            <tr key={ferramenta.name} className="align-top">
              <td className="py-2.5 pr-4">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="text-gray-200">{ferramenta.name}</span>
                  <span
                    className={`text-xs ${
                      uso === "unused" ? "text-gray-400" : "text-gray-300"
                    }`}
                  >
                    {ROTULO_USO[uso]}
                  </span>
                </div>
                {ferramenta.note && (
                  <p className="mt-0.5 max-w-md text-xs text-gray-400">
                    {ferramenta.note}
                  </p>
                )}
              </td>
              <td className="py-2.5 pr-4 whitespace-nowrap">
                {ferramenta.available ? (
                  <span
                    className={
                      uso === "unused" ? "text-gray-300" : "text-green-400"
                    }
                  >
                    disponível
                  </span>
                ) : uso === "required" ? (
                  <span className="text-red-400">ausente</span>
                ) : uso === "optional" ? (
                  <span className="text-amber-400">ausente</span>
                ) : (
                  <span className="text-gray-400">ausente</span>
                )}
              </td>
              <td className="py-2.5 text-gray-400 font-mono text-xs break-all">
                {ferramenta.version || "—"}
              </td>
            </tr>
          );
        })}
      </tbody>
    </table>
  </div>
);

const BlocoEtapa: React.FC<{ etapa: Etapa }> = ({ etapa }) => {
  const quando = formatarQuando(etapa.reported_at);

  return (
    <section className="bg-gray-800 border border-gray-700 rounded-xl p-5">
      <header className="flex flex-wrap items-start justify-between gap-3 mb-4">
        <div>
          <div className="flex items-center gap-3">
            <h2 className="text-base font-semibold text-gray-100">
              {etapa.label}
            </h2>
            <span
              className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-medium ${
                etapa.available
                  ? "bg-green-600/20 text-green-300 border border-green-700/50"
                  : etapa.stale
                    ? "bg-amber-600/20 text-amber-300 border border-amber-700/50"
                    : "bg-red-600/20 text-red-300 border border-red-700/50"
              }`}
            >
              {etapa.available ? (
                <CheckCircle size={12} />
              ) : etapa.stale ? (
                <Clock size={12} />
              ) : (
                <XCircle size={12} />
              )}
              {etapa.available
                ? "OK"
                : etapa.stale
                  ? "Sem relatório"
                  : "Indisponível"}
            </span>
          </div>
          <p className="mt-1 flex items-center gap-1.5 text-xs text-gray-400">
            <Cpu size={12} />
            {ORIGEM[etapa.source] ?? etapa.source}
            {quando && <span>· verificado {quando}</span>}
          </p>
        </div>
      </header>

      {etapa.detail && (
        <p
          className={`mb-4 text-sm ${
            etapa.stale ? "text-amber-300" : "text-red-300"
          }`}
        >
          {etapa.detail}
        </p>
      )}

      {etapa.providers.map((provider) => (
        <div
          key={`${etapa.role}-${provider.provider}`}
          className="mt-4 first:mt-0 rounded-lg border border-gray-700/70 bg-gray-900/40 p-4"
        >
          <div className="flex flex-wrap items-center justify-between gap-2 mb-3">
            <span className="text-sm font-medium text-gray-200">
              {provider.provider}
            </span>
            {provider.version && (
              <span className="text-xs text-gray-400 font-mono">
                {provider.version}
              </span>
            )}
          </div>
          {provider.detail && (
            <p className="mb-3 text-sm text-red-300">{provider.detail}</p>
          )}
          <TabelaFerramentas ferramentas={provider.tools ?? []} />
        </div>
      ))}
    </section>
  );
};

export const ProvidersPage: React.FC = () => {
  const { token } = useAuth();

  const { data, isLoading, isError } = useQuery({
    queryKey: ["admin", "providers"],
    queryFn: async (): Promise<Resposta> => {
      const res = await fetch(
        `${import.meta.env.VITE_API_URL}/v1/admin/providers`,
        { headers: { Authorization: `Bearer ${token}` } }
      );
      if (!res.ok) throw new Error("Falha ao consultar os providers.");
      return res.json();
    },
    enabled: !!token,
    // O diagnóstico executa os binários; revalidar de minuto em minuto basta.
    refetchInterval: 60_000,
  });

  const etapas = data?.stages ?? [];

  return (
    <AdminShell
      titulo="Mecanismo de download"
      descricao="Estado de cada etapa e das dependências que ela usa."
    >
      {isLoading && (
        <p className="flex items-center gap-2 text-gray-400">
          <Loader className="animate-spin" size={18} /> Consultando providers...
        </p>
      )}

      {isError && !isLoading && (
        <p className="text-red-400">Não foi possível consultar os providers.</p>
      )}

      {data && (
        <div className="space-y-6">
          <div
            className={`flex items-start gap-3 rounded-lg p-4 text-sm border ${
              data.healthy
                ? "bg-green-900/30 border-green-700 text-green-200"
                : "bg-red-900/30 border-red-700 text-red-200"
            }`}
          >
            {data.healthy ? (
              <CheckCircle size={18} className="mt-0.5 shrink-0" />
            ) : (
              <AlertTriangle size={18} className="mt-0.5 shrink-0" />
            )}
            <span>
              {data.healthy
                ? "Resolução e download estão operacionais."
                : "Há uma etapa com problema — veja qual delas abaixo."}
            </span>
          </div>

          <p className="text-sm text-gray-400">
            Resolver um link e baixá-lo acontecem em processos diferentes, com
            dependências diferentes: a API só lê metadados, e o ffmpeg — que
            junta vídeo e áudio — vive no worker, que é quem baixa.
          </p>

          {etapas.map((etapa) => (
            <BlocoEtapa key={etapa.role} etapa={etapa} />
          ))}

          {data.platforms && data.platforms.length > 0 && (
            <section className="bg-gray-800 border border-gray-700 rounded-xl p-5">
              <h2 className="text-base font-semibold text-gray-100 mb-2">
                Plataformas reconhecidas
              </h2>
              <p className="text-sm text-gray-400 mb-4">
                Estas têm rótulo próprio na tela. O provider aceita bem mais que
                isso — qualquer link público que ele saiba resolver funciona, só
                aparece como "Outra plataforma".
              </p>
              <div className="flex flex-wrap gap-2">
                {data.platforms.map((plataforma) => (
                  <span
                    key={plataforma.id}
                    className="px-2.5 py-1 rounded-full bg-gray-700 text-gray-300 text-xs"
                  >
                    {plataforma.label}
                  </span>
                ))}
              </div>
            </section>
          )}
        </div>
      )}
    </AdminShell>
  );
};
