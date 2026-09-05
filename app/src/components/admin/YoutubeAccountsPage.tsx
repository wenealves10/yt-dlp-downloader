import React, { useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  AlertTriangle,
  Loader,
  Monitor,
  Plus,
  RefreshCw,
  Repeat,
  ShieldCheck,
  Trash2,
  X,
} from "lucide-react";
import { AdminHeader } from "./AdminHeader";
import { AccountStatusBadge, BrowserStateBadge } from "./AccountStatusBadge";
import {
  useCheckYoutubeAccount,
  useCloseYoutubeBrowser,
  useCreateYoutubeAccount,
  useDeleteYoutubeAccount,
  useYoutubeAccounts,
} from "../../hooks/useYoutubeAccounts";
import type { YoutubeAccount } from "../../interface/YoutubeAccount";

function formatDate(value?: string): string {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "—";
  return date.toLocaleString("pt-BR", { dateStyle: "short", timeStyle: "short" });
}

// needsAuth é só para os estados em que falta o login do administrador. ERROR
// fica de fora de propósito: uma falha de infraestrutura não significa que a
// sessão expirou, e pedir um login novo nesse caso seria enganoso.
function needsAuth(account: YoutubeAccount): boolean {
  return (
    account.status === "REQUIRES_AUTH" || account.status === "NOT_CONFIGURED"
  );
}

export const YoutubeAccountsPage: React.FC = () => {
  const navigate = useNavigate();
  const { data, isLoading, isError, refetch, isFetching } = useYoutubeAccounts();
  const createAccount = useCreateYoutubeAccount();
  const deleteAccount = useDeleteYoutubeAccount();
  const checkAccount = useCheckYoutubeAccount();
  const closeBrowser = useCloseYoutubeBrowser();

  const [isFormOpen, setFormOpen] = useState(false);
  const [label, setLabel] = useState("");
  const [email, setEmail] = useState("");
  const [error, setError] = useState("");
  const [busyAccountID, setBusyAccountID] = useState<string | null>(null);

  const accounts = data?.accounts ?? [];
  const browserAvailable = data?.browser_available ?? false;
  const authenticatedCount = data?.authenticated_count ?? 0;

  const handleCreate = (event: React.FormEvent) => {
    event.preventDefault();
    setError("");

    createAccount.mutate(
      { label, email: email || undefined },
      {
        onSuccess: () => {
          setLabel("");
          setEmail("");
          setFormOpen(false);
        },
        onError: (mutationError) => setError(mutationError.message),
      }
    );
  };

  const runAction = (
    accountID: string,
    action: { mutate: (id: string, options?: object) => void }
  ) => {
    setError("");
    setBusyAccountID(accountID);
    action.mutate(accountID, {
      onError: (mutationError: Error) => setError(mutationError.message),
      onSettled: () => setBusyAccountID(null),
    });
  };

  const handleDelete = (account: YoutubeAccount) => {
    const confirmed = window.confirm(
      `Remover a conta "${account.label}"? A sessão autenticada será apagada permanentemente.`
    );
    if (!confirmed) return;
    runAction(account.id, deleteAccount);
  };

  return (
    <main className="bg-gray-900 text-white min-h-screen font-sans p-4 sm:p-6 lg:p-8">
      <div className="max-w-6xl mx-auto">
        <AdminHeader breadcrumb={["YouTube", "Contas"]} />

        <section className="mb-6 flex flex-wrap items-center justify-between gap-4">
          <div>
            <h1 className="text-2xl font-semibold text-gray-100">
              Contas do YouTube
            </h1>
            <p className="text-sm text-gray-400 mt-1">
              Sessões usadas pelo mecanismo de download. O login é feito
              manualmente no navegador remoto; o sistema nunca guarda a senha da
              conta.
            </p>
          </div>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => refetch()}
              disabled={isFetching}
              className="flex items-center gap-2 px-4 py-2 text-sm rounded-lg border border-gray-700 text-gray-300 hover:bg-gray-800 transition-colors disabled:opacity-50"
            >
              <RefreshCw
                size={16}
                className={isFetching ? "animate-spin" : ""}
              />
              Atualizar
            </button>
            <button
              type="button"
              onClick={() => setFormOpen((open) => !open)}
              className="flex items-center gap-2 bg-red-600 hover:bg-red-700 text-white font-medium px-4 py-2 rounded-lg transition-colors"
            >
              <Plus size={16} />
              Adicionar conta
            </button>
          </div>
        </section>

        {accounts.length > 0 && (
          <div className="mb-6 flex items-start gap-3 bg-gray-800 border border-gray-700 text-gray-300 rounded-lg p-4 text-sm">
            <Repeat size={18} className="mt-0.5 shrink-0 text-gray-500" />
            <p>
              {authenticatedCount === 0 ? (
                <>
                  Nenhuma conta autenticada no momento. Os downloads seguem sem
                  sessão gerenciada até que ao menos uma volte a autenticar.
                </>
              ) : (
                <>
                  <strong className="text-gray-100">
                    {authenticatedCount}
                  </strong>{" "}
                  {authenticatedCount === 1
                    ? "conta autenticada em uso."
                    : "contas autenticadas em rodízio."}{" "}
                  Cada download escolhe a conta usada há mais tempo. Se o
                  YouTube recusar uma sessão, ela sai do rodízio na hora e a
                  próxima conta assume automaticamente.
                </>
              )}
            </p>
          </div>
        )}

        {!browserAvailable && (
          <div className="mb-6 flex items-start gap-3 bg-yellow-900/30 border border-yellow-700 text-yellow-200 rounded-lg p-4 text-sm">
            <AlertTriangle size={18} className="mt-0.5 shrink-0" />
            <p>
              O serviço de navegador remoto não está respondendo. As contas
              continuam listadas, mas não é possível abrir sessões até que ele
              volte.
            </p>
          </div>
        )}

        {error && (
          <div className="mb-6 flex items-start justify-between gap-3 bg-red-900/30 border border-red-700 text-red-200 rounded-lg p-4 text-sm">
            <span>{error}</span>
            <button type="button" onClick={() => setError("")}>
              <X size={16} />
            </button>
          </div>
        )}

        {isFormOpen && (
          <form
            onSubmit={handleCreate}
            className="mb-6 bg-gray-800 border border-gray-700 rounded-xl p-6 space-y-4"
          >
            <div className="grid gap-4 sm:grid-cols-2">
              <div>
                <label className="block text-sm text-gray-400 mb-1">
                  Identificação da conta
                </label>
                <input
                  type="text"
                  value={label}
                  required
                  minLength={2}
                  maxLength={80}
                  onChange={(event) => setLabel(event.target.value)}
                  placeholder="Conta principal"
                  className="w-full bg-gray-900 border border-gray-600 rounded-lg py-2.5 px-3 focus:ring-2 focus:ring-red-500 focus:border-red-500 transition-all placeholder-gray-500"
                />
              </div>
              <div>
                <label className="block text-sm text-gray-400 mb-1">
                  E-mail (opcional)
                </label>
                <input
                  type="email"
                  value={email}
                  onChange={(event) => setEmail(event.target.value)}
                  placeholder="conta@gmail.com"
                  className="w-full bg-gray-900 border border-gray-600 rounded-lg py-2.5 px-3 focus:ring-2 focus:ring-red-500 focus:border-red-500 transition-all placeholder-gray-500"
                />
                <p className="text-xs text-gray-500 mt-1">
                  Serve apenas para identificação. Nenhuma senha é solicitada ou
                  armazenada.
                </p>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <button
                type="submit"
                disabled={createAccount.isPending || !label}
                className="flex items-center gap-2 bg-red-600 hover:bg-red-700 disabled:bg-red-800 disabled:cursor-not-allowed text-white font-medium px-4 py-2 rounded-lg transition-colors"
              >
                {createAccount.isPending ? (
                  <>
                    <Loader className="animate-spin" size={16} /> Preparando
                    sessão...
                  </>
                ) : (
                  <>
                    <Plus size={16} /> Criar conta
                  </>
                )}
              </button>
              <button
                type="button"
                onClick={() => setFormOpen(false)}
                className="px-4 py-2 text-sm rounded-lg border border-gray-700 text-gray-300 hover:bg-gray-700 transition-colors"
              >
                Cancelar
              </button>
            </div>
          </form>
        )}

        {isLoading && (
          <div className="flex items-center gap-2 text-gray-400">
            <Loader className="animate-spin" size={18} /> Carregando contas...
          </div>
        )}

        {isError && !isLoading && (
          <p className="text-red-400">Não foi possível carregar as contas.</p>
        )}

        {!isLoading && accounts.length === 0 && (
          <div className="bg-gray-800 border border-dashed border-gray-700 rounded-xl p-10 text-center">
            <Monitor className="mx-auto mb-3 text-gray-600" size={32} />
            <p className="text-gray-300 font-medium">
              Nenhuma conta cadastrada
            </p>
            <p className="text-sm text-gray-500 mt-1">
              Clique em "Adicionar conta" para preparar a primeira sessão.
            </p>
          </div>
        )}

        <div className="space-y-4">
          {accounts.map((account) => {
            const busy = busyAccountID === account.id;
            const browserRunning = account.browser_state === "RUNNING";

            return (
              <article
                key={account.id}
                className="bg-gray-800 border border-gray-700 rounded-xl p-5"
              >
                <div className="flex flex-wrap items-start justify-between gap-4">
                  <div className="min-w-0">
                    <h2 className="text-lg font-semibold text-gray-100 truncate">
                      {account.label}
                    </h2>
                    <p className="text-sm text-gray-400 truncate">
                      {account.email || "E-mail não identificado"}
                    </p>
                    <div className="flex flex-wrap items-center gap-2 mt-3">
                      <AccountStatusBadge status={account.status} />
                      <BrowserStateBadge state={account.browser_state} />
                      {account.next_in_rotation && (
                        <span className="inline-flex items-center gap-1 text-xs font-medium px-2.5 py-0.5 rounded-full bg-gray-700 text-gray-200">
                          <Repeat size={12} />
                          Próxima do rodízio
                        </span>
                      )}
                      {!account.active && (
                        <span className="text-xs text-gray-500">
                          conta desativada
                        </span>
                      )}
                    </div>
                  </div>

                  <dl className="grid grid-cols-2 gap-x-6 gap-y-1 text-xs text-gray-400">
                    <dt>Última verificação</dt>
                    <dd className="text-gray-300">
                      {formatDate(account.last_checked_at)}
                    </dd>
                    <dt>Última autenticação</dt>
                    <dd className="text-gray-300">
                      {formatDate(account.last_authenticated_at)}
                    </dd>
                    <dt>Último download</dt>
                    <dd className="text-gray-300">
                      {formatDate(account.last_used_at)}
                    </dd>
                    <dt>Última atividade</dt>
                    <dd className="text-gray-300">
                      {formatDate(
                        account.browser_activity_at || account.last_used_at
                      )}
                    </dd>
                  </dl>
                </div>

                {needsAuth(account) && (
                  <p className="mt-4 flex items-start gap-2 text-sm text-yellow-300">
                    <AlertTriangle size={16} className="mt-0.5 shrink-0" />
                    <span>
                      Esta conta precisa ser autenticada novamente.
                      {account.last_error && (
                        <span className="text-yellow-500/80">
                          {" "}
                          ({account.last_error})
                        </span>
                      )}
                    </span>
                  </p>
                )}

                {account.status === "ERROR" && (
                  <p className="mt-4 flex items-start gap-2 text-sm text-red-300">
                    <AlertTriangle size={16} className="mt-0.5 shrink-0" />
                    <span>
                      Não foi possível verificar esta conta.
                      {account.last_error && (
                        <span className="text-red-400/80"> {account.last_error}.</span>
                      )}{" "}
                      A sessão salva continua intacta.
                    </span>
                  </p>
                )}

                <div className="mt-5 flex flex-wrap items-center gap-2">
                  <button
                    type="button"
                    disabled={!browserAvailable || busy}
                    onClick={() =>
                      navigate(`/admin/youtube/accounts/${account.id}/browser`)
                    }
                    className="flex items-center gap-2 bg-red-600 hover:bg-red-700 disabled:bg-gray-700 disabled:cursor-not-allowed text-white text-sm font-medium px-4 py-2 rounded-lg transition-colors"
                  >
                    <Monitor size={16} />
                    {needsAuth(account)
                      ? "Reautenticar"
                      : browserRunning
                        ? "Ver navegador"
                        : "Abrir navegador"}
                  </button>

                  <button
                    type="button"
                    disabled={!browserAvailable || busy}
                    onClick={() => runAction(account.id, checkAccount)}
                    className="flex items-center gap-2 border border-gray-700 text-gray-300 hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed text-sm px-4 py-2 rounded-lg transition-colors"
                  >
                    {busy ? (
                      <Loader className="animate-spin" size={16} />
                    ) : (
                      <ShieldCheck size={16} />
                    )}
                    Verificar sessão
                  </button>

                  {browserRunning && (
                    <button
                      type="button"
                      disabled={busy}
                      onClick={() => runAction(account.id, closeBrowser)}
                      className="flex items-center gap-2 border border-gray-700 text-gray-300 hover:bg-gray-700 disabled:opacity-50 text-sm px-4 py-2 rounded-lg transition-colors"
                    >
                      <X size={16} />
                      Fechar navegador
                    </button>
                  )}

                  <button
                    type="button"
                    disabled={busy}
                    onClick={() => handleDelete(account)}
                    className="flex items-center gap-2 text-red-400 hover:bg-gray-700 disabled:opacity-50 text-sm px-4 py-2 rounded-lg transition-colors ml-auto"
                  >
                    <Trash2 size={16} />
                    Remover
                  </button>
                </div>
              </article>
            );
          })}
        </div>
      </div>
    </main>
  );
};
