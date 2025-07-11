import { useEffect, useState } from "react";
import { Navigate, Outlet } from "react-router-dom";
import { useAuth } from "../../hooks/useAuth";
import Loading from "../loading/Loading";

export default function PrivateRoute() {
  const { token, setUser } = useAuth();
  const [loading, setLoading] = useState(true);
  const [isValid, setIsValid] = useState(false);

  useEffect(() => {
    const verifyToken = async () => {
      if (!token) {
        setLoading(false);
        return;
      }

      try {
        const response = await fetch(
          `${import.meta.env.VITE_API_URL}/v1/profile`,
          {
            method: "GET",
            headers: {
              "Content-Type": "application/json",
              Authorization: `Bearer ${token}`,
            },
          }
        );

        if (response.ok) {
          const userData = await response.json();
          setUser(userData);
          setIsValid(true);
        } else {
          localStorage.removeItem("authToken");
        }
      } catch (err) {
        console.error("Erro ao verificar token:", err);
        localStorage.removeItem("authToken");
      } finally {
        setLoading(false);
      }
    };

    verifyToken();
  }, [token, setUser]);

  if (loading) return <Loading />;
  if (!token || !isValid) return <Navigate to="/" replace />;

  return <Outlet />;
}
