import { createContext, useState, useEffect } from "react";
import { useNavigate, useLocation } from "react-router-dom";
import type { User } from "../interface/User";
import Loading from "../components/loading/Loading";

type AuthContextType = {
  token: string | null;
  login: (token: string) => void;
  logout: () => void;
  user: User | null;
  setUser: (user: User | null) => void;
};

export const AuthContext = createContext<AuthContextType | null>(null);

export const AuthProvider = ({ children }: { children: React.ReactNode }) => {
  const [token, setToken] = useState<string | null>(() =>
    localStorage.getItem("token")
  );
  const [user, setUser] = useState<User | null>(null);
  const [isChecking, setIsChecking] = useState(true);

  const navigate = useNavigate();
  const location = useLocation();

  const login = (newToken: string) => {
    localStorage.setItem("token", newToken);
    setToken(newToken);
  };

  const logout = () => {
    localStorage.removeItem("token");
    setToken(null);
    setUser(null);
    navigate("/", { replace: true });
  };

  // Verificação automática do token ao iniciar
  useEffect(() => {
    const verifyToken = async () => {
      if (!token) {
        setIsChecking(false);
        if (location.pathname !== "/") navigate("/", { replace: true });
        return;
      }

      try {
        const response = await fetch(
          `${import.meta.env.VITE_API_URL}/v1/profile`,
          {
            headers: {
              Authorization: `Bearer ${token}`,
              "Content-Type": "application/json",
            },
          }
        );

        if (!response.ok) {
          throw new Error("Token inválido");
        }

        const userData = await response.json();
        setUser(userData);

        if (location.pathname === "/") {
          navigate("/dashboard", { replace: true });
        }
      } catch (err) {
        logout();
      } finally {
        setIsChecking(false);
      }
    };

    verifyToken();
  }, [token]);

  // Enquanto verifica, pode exibir um loading global (opcional)
  if (isChecking) return <Loading />;

  return (
    <AuthContext.Provider value={{ token, login, logout, user, setUser }}>
      {children}
    </AuthContext.Provider>
  );
};
