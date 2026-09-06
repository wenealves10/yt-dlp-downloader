import React from "react";
import { Link, useLocation } from "react-router-dom";
import { ArrowLeft, BarChart3, Download, Server, Users, Youtube } from "lucide-react";
import { UserMenu } from "../user/UserMenu";
import { useAuth } from "../../hooks/useAuth";

const ABAS = [
  { to: "/admin", rotulo: "Dashboard", icone: BarChart3, exato: true },
  { to: "/admin/users", rotulo: "Usuários", icone: Users, exato: false },
  { to: "/admin/downloads", rotulo: "Downloads", icone: Download, exato: false },
  { to: "/admin/youtube/accounts", rotulo: "YouTube", icone: Youtube, exato: false },
  { to: "/admin/providers", rotulo: "Providers", icone: Server, exato: false },
];

interface Props {
  titulo: string;
  descricao?: string;
  acoes?: React.ReactNode;
  children: React.ReactNode;
}

// Casca comum das telas administrativas: a navegação some se cada tela montar a
// sua, e o administrador perde a noção de onde está.
export const AdminShell: React.FC<Props> = ({ titulo, descricao, acoes, children }) => {
  const { user, logout } = useAuth();
  const { pathname } = useLocation();

  return (
    <main className="bg-gray-900 text-white min-h-screen font-sans">
      <div className="border-b border-gray-800">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex items-center justify-between h-16">
            <div className="flex items-center gap-4">
              <Link
                to="/dashboard"
                className="flex items-center gap-2 text-sm text-gray-400 hover:text-white transition-colors"
              >
                <ArrowLeft size={16} />
                <span className="hidden sm:inline">Voltar ao app</span>
              </Link>
              <span className="text-gray-700">/</span>
              <span className="text-sm font-medium text-gray-200">Administração</span>
            </div>
            <UserMenu user={user} onLogout={logout} />
          </div>

          <nav className="flex gap-1 -mb-px overflow-x-auto">
            {ABAS.map(({ to, rotulo, icone: Icone, exato }) => {
              const ativa = exato ? pathname === to : pathname.startsWith(to);
              return (
                <Link
                  key={to}
                  to={to}
                  className={`flex items-center gap-2 px-4 py-3 text-sm font-medium border-b-2 whitespace-nowrap transition-colors ${
                    ativa
                      ? "border-red-500 text-white"
                      : "border-transparent text-gray-400 hover:text-gray-200"
                  }`}
                >
                  <Icone size={16} />
                  {rotulo}
                </Link>
              );
            })}
          </nav>
        </div>
      </div>

      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <header className="flex flex-wrap items-start justify-between gap-4 mb-6">
          <div>
            <h1 className="text-2xl font-semibold text-gray-100">{titulo}</h1>
            {descricao && <p className="text-sm text-gray-400 mt-1">{descricao}</p>}
          </div>
          {acoes && <div className="flex flex-wrap items-center gap-2">{acoes}</div>}
        </header>
        {children}
      </div>
    </main>
  );
};
