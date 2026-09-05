import { Navigate, Outlet } from "react-router-dom";
import { useAuth } from "../../hooks/useAuth";
import Loading from "../loading/Loading";

// Guarda de rota do painel administrativo. É apenas a camada de conveniência do
// frontend: a autorização real acontece na API, que recusa qualquer requisição
// de quem não for super admin.
export default function SuperAdminRoute() {
  const { token, user } = useAuth();

  if (!token) return <Navigate to="/" replace />;
  if (!user) return <Loading />;
  if (user.role !== "super_admin") return <Navigate to="/dashboard" replace />;

  return <Outlet />;
}
