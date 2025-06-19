import { createContext, useState } from "react";
import type { User } from "../interface/User";

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

  const login = (token: string) => {
    localStorage.setItem("token", token);
    setToken(token);
  };

  const logout = () => {
    localStorage.removeItem("token");
    setToken(null);
  };

  return (
    <AuthContext.Provider value={{ token, login, logout, user, setUser }}>
      {children}
    </AuthContext.Provider>
  );
};
