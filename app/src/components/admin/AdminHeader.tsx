import React from "react";
import { Link } from "react-router-dom";
import { ArrowLeft } from "lucide-react";
import { UserMenu } from "../user/UserMenu";
import { useAuth } from "../../hooks/useAuth";

interface AdminHeaderProps {
  breadcrumb: string[];
  backTo?: string;
  backLabel?: string;
}

// Cabeçalho enxuto, usado apenas pela tela do navegador remoto: ali a navegação
// completa do painel competiria com a janela do Chrome. As demais telas
// administrativas usam o AdminShell.
export const AdminHeader: React.FC<AdminHeaderProps> = ({
  breadcrumb,
  backTo = "/dashboard",
  backLabel = "Voltar",
}) => {
  const { user, logout } = useAuth();

  return (
    <header className="mb-8 flex flex-wrap justify-between items-center gap-4">
      <div className="flex items-center gap-4">
        <Link
          to={backTo}
          className="flex items-center gap-2 text-sm text-gray-400 hover:text-white transition-colors"
        >
          <ArrowLeft size={16} />
          {backLabel}
        </Link>
        <nav className="text-sm text-gray-500">
          {breadcrumb.map((item, index) => (
            <span key={item}>
              {index > 0 && <span className="mx-2 text-gray-700">/</span>}
              <span
                className={
                  index === breadcrumb.length - 1
                    ? "text-gray-200 font-medium"
                    : ""
                }
              >
                {item}
              </span>
            </span>
          ))}
        </nav>
      </div>
      <UserMenu user={user} onLogout={logout} />
    </header>
  );
};
