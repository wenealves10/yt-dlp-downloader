import React from "react";
import { useQuery } from "@tanstack/react-query";
import { AlertTriangle, CheckCircle, Loader, XCircle } from "lucide-react";
import { AdminShell } from "./AdminShell";
import { useAuth } from "../../hooks/useAuth";

interface Ferramenta {
  name: string;
  available: boolean;
  version?: string;
  required: boolean;
}

interface ProviderSaude {
  provider: string;
  available: boolean;
  version?: string;
  detail?: string;
  tools?: Ferramenta[];
  checked_at: string;
}

interface Resposta {
  providers: ProviderSaude[];
  healthy: boolean;
  detail?: string;
  platforms?: { id: string; label: string }[];
}

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

  return (
    <AdminShell
      titulo="Mecanismo de download"
      descricao="Estado dos providers e das dependências que eles usam."
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
                ? "Todos os providers estão operacionais."
                : "Há provider indisponível — downloads podem falhar."}
              {data.detail && <span className="block mt-0.5">{data.detail}</span>}
            </span>
          </div>

          {data.providers.map((provider) => (
            <section
              key={provider.provider}
              className="bg-gray-800 border border-gray-700 rounded-xl p-5"
            >
              <header className="flex flex-wrap items-center justify-between gap-3 mb-4">
                <div className="flex items-center gap-3">
                  <h2 className="text-base font-semibold text-gray-100">
                    {provider.provider}
                  </h2>
                  <span
                    className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-medium ${
                      provider.available
                        ? "bg-green-600/20 text-green-300 border border-green-700/50"
                        : "bg-red-600/20 text-red-300 border border-red-700/50"
                    }`}
                  >
                    {provider.available ? (
                      <CheckCircle size={12} />
                    ) : (
                      <XCircle size={12} />
                    )}
                    {provider.available ? "OK" : "Indisponível"}
                  </span>
                </div>
                {provider.version && (
                  <span className="text-sm text-gray-400 font-mono">
                    {provider.version}
                  </span>
                )}
              </header>

              {provider.detail && (
                <p className="mb-4 text-sm text-red-300">{provider.detail}</p>
              )}

              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="text-left text-xs text-gray-500 uppercase tracking-wide">
                      <th className="pb-2 font-medium">Dependência</th>
                      <th className="pb-2 font-medium">Estado</th>
                      <th className="pb-2 font-medium">Versão</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-700">
                    {(provider.tools ?? []).map((ferramenta) => (
                      <tr key={ferramenta.name}>
                        <td className="py-2.5 pr-4 text-gray-200">
                          {ferramenta.name}
                          {!ferramenta.required && (
                            <span className="ml-2 text-xs text-gray-500">
                              opcional
                            </span>
                          )}
                        </td>
                        <td className="py-2.5 pr-4">
                          {ferramenta.available ? (
                            <span className="text-green-400">disponível</span>
                          ) : (
                            <span
                              className={
                                ferramenta.required
                                  ? "text-red-400"
                                  : "text-amber-400"
                              }
                            >
                              ausente
                            </span>
                          )}
                        </td>
                        <td className="py-2.5 text-gray-400 font-mono text-xs">
                          {ferramenta.version || "—"}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </section>
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
