import React, { useState } from "react";
import { Youtube, Mail, Lock } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { useMutation } from "@tanstack/react-query";
import { login, type LoginPayload } from "../../api/auth";
import { useAuth } from "../../hooks/useAuth";
import { Turnstile } from "../turnstile/Turnstile";

export const LoginPage: React.FC = () => {
  const navigate = useNavigate();
  const { login: loginContext } = useAuth(); // pegando a função login do contexto

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [token, setToken] = useState<string>("");

  const loginMutation = useMutation({
    mutationFn: (payload: LoginPayload) => login(payload),
    onSuccess: (data) => {
      loginContext(data.access_token); // salva no contexto + localStorage
      navigate("/dashboard"); // redireciona
    },
    onError: () => {
      alert("Email ou senha inválidos");
    },
  });

  const onLogin = () => {
    loginMutation.mutate({ email, password, turnstileToken: token });
  };

  const onSwitchToRegister = () => {
    navigate("/register");
  };

  return (
    <div className="bg-gray-900 min-h-screen flex items-center justify-center p-4 font-sans">
      <div className="w-full max-w-md">
        <header className="text-center mb-8">
          <div className="flex justify-center items-center gap-4">
            <Youtube className="h-12 w-12 text-red-600" />
            <div>
              <h1 className="text-4xl sm:text-5xl font-bold tracking-tight bg-gradient-to-r from-red-500 to-red-700 text-transparent bg-clip-text">
                AdVideo
              </h1>
              <p className="text-gray-400 mt-1">
                Acesse sua conta para continuar
              </p>
            </div>
          </div>
        </header>
        <main className="bg-gray-800 p-8 rounded-xl shadow-2xl border border-gray-700">
          <form
            onSubmit={(e) => {
              e.preventDefault();
              onLogin();
            }}
            className="space-y-4"
          >
            <div className="relative">
              <Mail className="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-gray-500" />
              <input
                type="email"
                placeholder="email@exemplo.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                className="w-full bg-gray-900 border border-gray-600 rounded-lg py-3 pl-10 pr-4 focus:ring-2 focus:ring-red-500 focus:border-red-500 transition-all placeholder-gray-500 text-white"
              />
            </div>
            <div className="relative">
              <Lock className="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-gray-500" />
              <input
                type="password"
                placeholder="Sua senha"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                className="w-full bg-gray-900 border border-gray-600 rounded-lg py-3 pl-10 pr-4 focus:ring-2 focus:ring-red-500 focus:border-red-500 transition-all placeholder-gray-500 text-white"
              />
            </div>
            <div className="w-full flex items-center justify-center mt-4">
              <Turnstile
                siteKey={import.meta.env.VITE_SITE_KEY || ""}
                onVerify={setToken}
              />
            </div>
            <button
              type="submit"
              className="w-full flex items-center justify-center gap-2 bg-red-600 hover:bg-red-700 text-white font-bold py-3 px-6 rounded-lg transition-all"
              disabled={loginMutation.isPending || !token}
            >
              {loginMutation.isPending ? "Entrando..." : "Entrar"}
            </button>
          </form>
          <p className="text-center text-sm text-gray-400 mt-6">
            Não tem uma conta?{" "}
            <button
              onClick={onSwitchToRegister}
              className="font-medium text-red-500 hover:underline bg-transparent border-none cursor-pointer"
            >
              Cadastre-se
            </button>
          </p>
        </main>
      </div>
    </div>
  );
};
