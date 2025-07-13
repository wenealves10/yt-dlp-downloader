import { createContext, useState, useEffect } from "react";
import { useNavigate, useLocation } from "react-router-dom";
import type { User } from "../interface/User";
import Loading from "../components/loading/Loading";
import { getProfile } from "../api/auth";

type AuthContextType = {
  token: string | null;
  login: (token: string) => void;
  logout: () => void;
  user: User | null;
  setUser: (user: User | null) => void;
  refreshProfile: () => Promise<void>;
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

  const refreshProfile = async () => {
    if (!token) {
      setIsChecking(false);
      if (location.pathname !== "/") navigate("/", { replace: true });
      return;
    }
    try {
      const dataProfile = await getProfile(token);
      setUser(dataProfile);
      if (location.pathname === "/") {
        navigate("/dashboard", { replace: true });
      }
    } catch (err) {
      logout();
    } finally {
      setIsChecking(false);
    }
  };

  useEffect(() => {
    refreshProfile();
  }, [token]);

  if (isChecking) return <Loading />;

  return (
    <AuthContext.Provider
      value={{ token, login, logout, user, setUser, refreshProfile }}
    >
      {children}
    </AuthContext.Provider>
  );
};
