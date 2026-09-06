import React, { useEffect, useState } from "react";
import { Check, Copy, KeyRound, Loader, X } from "lucide-react";
import type { AdminUser, Plan, Role } from "../../interface/Admin";
import {
  useCreateUser,
  useResetPassword,
  useUpdateUser,
} from "../../hooks/useAdmin";

interface Props {
  usuario: AdminUser | null; // null = criação
  aoFechar: () => void;
}

const PLANOS: Plan[] = ["free", "premium", "enterprise"];
const PAPEIS: Role[] = ["user", "admin", "super_admin"];

// SenhaGerada mostra a senha uma única vez. Depois de fechar o modal ela não
// existe mais em lugar nenhum: o banco guarda só o hash.
const SenhaGerada: React.FC<{ senha: string }> = ({ senha }) => {
  const [copiada, setCopiada] = useState(false);

  const copiar = async () => {
    try {
      await navigator.clipboard.writeText(senha);
      setCopiada(true);
      setTimeout(() => setCopiada(false), 2000);
    } catch {
      // Sem permissão de área de transferência: a senha continua visível na
      // tela para ser copiada à mão.
    }
  };

  return (
    <div className="bg-amber-900/30 border border-amber-700 rounded-lg p-4">
      <p className="text-sm text-amber-200 font-medium mb-2">
        Senha gerada — anote agora
      </p>
      <div className="flex items-center gap-2">
        <code className="flex-1 bg-gray-950 border border-gray-700 rounded-md px-3 py-2 text-base text-gray-100 font-mono select-all">
          {senha}
        </code>
        <button
          type="button"
          onClick={copiar}
          className="flex items-center gap-1.5 px-3 py-2 rounded-md border border-gray-700 text-sm text-gray-300 hover:bg-gray-700 transition-colors"
        >
          {copiada ? <Check size={16} /> : <Copy size={16} />}
          {copiada ? "Copiado" : "Copiar"}
        </button>
      </div>
      <p className="text-xs text-amber-400/80 mt-2">
        Ela não pode ser consultada depois: o sistema guarda apenas o hash. Se
        perder, gere outra.
      </p>
    </div>
  );
};

export const UserFormModal: React.FC<Props> = ({ usuario, aoFechar }) => {
  const editando = usuario !== null;

  const [fullName, setFullName] = useState(usuario?.full_name ?? "");
  const [email, setEmail] = useState(usuario?.email ?? "");
  const [plan, setPlan] = useState<Plan>(usuario?.plan ?? "free");
  const [role, setRole] = useState<Role>(usuario?.role ?? "user");
  const [dailyLimit, setDailyLimit] = useState(usuario?.daily_limit ?? 2);
  const [active, setActive] = useState(usuario?.active ?? true);
  const [senhaManual, setSenhaManual] = useState("");
  const [senhaGerada, setSenhaGerada] = useState<string | null>(null);
  const [erro, setErro] = useState("");

  const criar = useCreateUser();
  const atualizar = useUpdateUser();
  const redefinir = useResetPassword();
  const salvando = criar.isPending || atualizar.isPending;

  // Fechar com Esc é o reflexo de quem usa o painel o dia inteiro.
  useEffect(() => {
    const aoTeclar = (evento: KeyboardEvent) => {
      if (evento.key === "Escape") aoFechar();
    };
    document.addEventListener("keydown", aoTeclar);
    return () => document.removeEventListener("keydown", aoTeclar);
  }, [aoFechar]);

  const salvar = (evento: React.FormEvent) => {
    evento.preventDefault();
    setErro("");

    if (editando) {
      atualizar.mutate(
        {
          id: usuario.id,
          full_name: fullName,
          email,
          plan,
          role,
          daily_limit: dailyLimit,
          active,
        },
        {
          onSuccess: aoFechar,
          onError: (falha) => setErro(falha.message),
        }
      );
      return;
    }

    criar.mutate(
      {
        full_name: fullName,
        email,
        plan,
        role,
        daily_limit: dailyLimit,
        password: senhaManual || undefined,
      },
      {
        onSuccess: (resposta) => {
          if (resposta.generated_password) {
            // Mantém o modal aberto: fechar aqui perderia a senha para sempre.
            setSenhaGerada(resposta.generated_password);
          } else {
            aoFechar();
          }
        },
        onError: (falha) => setErro(falha.message),
      }
    );
  };

  const gerarNovaSenha = () => {
    if (!usuario) return;
    setErro("");
    redefinir.mutate(
      { id: usuario.id },
      {
        onSuccess: (resposta) => setSenhaGerada(resposta.generated_password ?? null),
        onError: (falha) => setErro(falha.message),
      }
    );
  };

  return (
    <div className="fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center z-50 p-4 overflow-y-auto">
      <div className="bg-gray-800 w-full max-w-lg rounded-xl shadow-2xl border border-gray-700 my-8">
        <header className="p-4 border-b border-gray-700 flex justify-between items-center">
          <h2 className="text-lg font-semibold text-white">
            {editando ? "Editar usuário" : "Novo usuário"}
          </h2>
          <button onClick={aoFechar} className="text-gray-400 hover:text-white">
            <X size={20} />
          </button>
        </header>

        {senhaGerada ? (
          <div className="p-6 space-y-4">
            <SenhaGerada senha={senhaGerada} />
            <button
              type="button"
              onClick={aoFechar}
              className="w-full bg-red-600 hover:bg-red-700 text-white font-medium py-2.5 rounded-lg transition-colors"
            >
              Concluir
            </button>
          </div>
        ) : (
          <form onSubmit={salvar} className="p-6 space-y-4">
            {erro && (
              <p className="bg-red-900/30 border border-red-700 text-red-200 rounded-lg p-3 text-sm">
                {erro}
              </p>
            )}

            <div>
              <label className="block text-sm text-gray-400 mb-1">Nome</label>
              <input
                type="text"
                required
                minLength={2}
                value={fullName}
                onChange={(evento) => setFullName(evento.target.value)}
                className="w-full bg-gray-900 border border-gray-600 rounded-lg py-2.5 px-3 focus:ring-2 focus:ring-red-500 focus:border-red-500 transition-all"
              />
            </div>

            <div>
              <label className="block text-sm text-gray-400 mb-1">E-mail</label>
              <input
                type="email"
                required
                value={email}
                onChange={(evento) => setEmail(evento.target.value)}
                className="w-full bg-gray-900 border border-gray-600 rounded-lg py-2.5 px-3 focus:ring-2 focus:ring-red-500 focus:border-red-500 transition-all"
              />
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-sm text-gray-400 mb-1">Plano</label>
                <select
                  value={plan}
                  onChange={(evento) => setPlan(evento.target.value as Plan)}
                  className="w-full bg-gray-900 border border-gray-600 rounded-lg py-2.5 px-3 text-gray-100"
                >
                  {PLANOS.map((opcao) => (
                    <option key={opcao} value={opcao}>
                      {opcao}
                    </option>
                  ))}
                </select>
              </div>
              <div>
                <label className="block text-sm text-gray-400 mb-1">Papel</label>
                <select
                  value={role}
                  onChange={(evento) => setRole(evento.target.value as Role)}
                  className="w-full bg-gray-900 border border-gray-600 rounded-lg py-2.5 px-3 text-gray-100"
                >
                  {PAPEIS.map((opcao) => (
                    <option key={opcao} value={opcao}>
                      {opcao}
                    </option>
                  ))}
                </select>
              </div>
            </div>

            <div>
              <label className="block text-sm text-gray-400 mb-1">
                Downloads por dia
              </label>
              <input
                type="number"
                min={0}
                max={100000}
                value={dailyLimit}
                onChange={(evento) => setDailyLimit(Number(evento.target.value))}
                className="w-full bg-gray-900 border border-gray-600 rounded-lg py-2.5 px-3"
              />
              <p className="text-xs text-gray-500 mt-1">
                O super admin não tem teto, qualquer que seja este valor.
              </p>
            </div>

            {!editando && (
              <div>
                <label className="block text-sm text-gray-400 mb-1">
                  Senha (opcional)
                </label>
                <input
                  type="text"
                  minLength={8}
                  value={senhaManual}
                  onChange={(evento) => setSenhaManual(evento.target.value)}
                  placeholder="Deixe vazio para o sistema gerar"
                  className="w-full bg-gray-900 border border-gray-600 rounded-lg py-2.5 px-3 placeholder-gray-600"
                />
              </div>
            )}

            {editando && (
              <>
                <label className="flex items-center gap-3 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={active}
                    onChange={(evento) => setActive(evento.target.checked)}
                    className="w-4 h-4 accent-red-600"
                  />
                  <span className="text-sm text-gray-300">
                    Conta ativa
                    <span className="text-gray-500">
                      {" "}
                      — desmarcar bloqueia o login
                    </span>
                  </span>
                </label>

                <button
                  type="button"
                  onClick={gerarNovaSenha}
                  disabled={redefinir.isPending}
                  className="w-full flex items-center justify-center gap-2 border border-gray-600 text-gray-300 hover:bg-gray-700 disabled:opacity-50 py-2.5 rounded-lg text-sm transition-colors"
                >
                  {redefinir.isPending ? (
                    <Loader className="animate-spin" size={16} />
                  ) : (
                    <KeyRound size={16} />
                  )}
                  Gerar nova senha
                </button>
              </>
            )}

            <div className="flex gap-2 pt-2">
              <button
                type="submit"
                disabled={salvando}
                className="flex-1 flex items-center justify-center gap-2 bg-red-600 hover:bg-red-700 disabled:bg-red-800 text-white font-medium py-2.5 rounded-lg transition-colors"
              >
                {salvando && <Loader className="animate-spin" size={16} />}
                {editando ? "Salvar" : "Criar usuário"}
              </button>
              <button
                type="button"
                onClick={aoFechar}
                className="px-4 py-2.5 rounded-lg border border-gray-700 text-gray-300 hover:bg-gray-700 text-sm transition-colors"
              >
                Cancelar
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  );
};
