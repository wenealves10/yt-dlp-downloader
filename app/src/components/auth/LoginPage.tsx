import React from "react";
import { Youtube, Mail, Lock, Github } from "lucide-react";

const GoogleIcon = () => (
  <svg className="h-5 w-5" viewBox="0 0 48 48">
    <path
      fill="#FFC107"
      d="M43.611,20.083H42V20H24v8h11.303c-1.649,4.657-6.08,8-11.303,8c-6.627,0-12-5.373-12-12s5.373-12,12-12c3.059,0,5.842,1.154,7.961,3.039l5.657-5.657C34.046,6.053,29.268,4,24,4C12.955,4,4,12.955,4,24s8.955,20,20,20s20-8.955,20-20C44,22.659,43.862,21.35,43.611,20.083z"
    ></path>
    <path
      fill="#FF3D00"
      d="M6.306,14.691l6.571,4.819C14.655,15.108,18.961,12,24,12c3.059,0,5.842,1.154,7.961,3.039l5.657-5.657C34.046,6.053,29.268,4,24,4C16.318,4,9.656,8.337,6.306,14.691z"
    ></path>
    <path
      fill="#4CAF50"
      d="M24,44c5.166,0,9.86-1.977,13.409-5.192l-6.19-5.238C29.211,35.091,26.715,36,24,36c-5.202,0-9.619-3.317-11.283-7.946l-6.522,5.025C9.505,39.556,16.227,44,24,44z"
    ></path>
    <path
      fill="#1976D2"
      d="M43.611,20.083H42V20H24v8h11.303c-0.792,2.237-2.231,4.166-4.087,5.574l6.19,5.238C42.021,35.591,44,30.138,44,24C44,22.659,43.862,21.35,43.611,20.083z"
    ></path>
  </svg>
);

interface LoginPageProps {
  onLogin: () => void;
  onSwitchToRegister: () => void;
}

export const LoginPage: React.FC<LoginPageProps> = ({
  onLogin,
  onSwitchToRegister,
}) => (
  <div className="bg-gray-900 min-h-screen flex items-center justify-center p-4 font-sans">
    <div className="w-full max-w-md">
      <header className="text-center mb-8">
        <div className="flex justify-center items-center gap-4">
          <Youtube className="h-12 w-12 text-red-600" />
          <div>
            <h1 className="text-4xl sm:text-5xl font-bold tracking-tight bg-gradient-to-r from-red-500 to-red-700 text-transparent bg-clip-text">
              YT Downloader
            </h1>
            <p className="text-gray-400 mt-1">
              Acesse sua conta para continuar
            </p>
          </div>
        </div>
      </header>
      <main className="bg-gray-800 p-8 rounded-xl shadow-2xl border border-gray-700">
        <div className="space-y-4">
          <button
            onClick={onLogin}
            className="w-full flex items-center justify-center gap-3 bg-gray-700 hover:bg-gray-600 text-white font-bold py-3 px-4 rounded-lg transition-colors"
          >
            <GoogleIcon />
            <span>Entrar com Google</span>
          </button>
          <button
            onClick={onLogin}
            className="w-full flex items-center justify-center gap-3 bg-gray-700 hover:bg-gray-600 text-white font-bold py-3 px-4 rounded-lg transition-colors"
          >
            <Github className="h-5 w-5" />
            <span>Entrar com GitHub</span>
          </button>
        </div>
        <div className="my-6 flex items-center">
          <div className="flex-grow border-t border-gray-600"></div>
          <span className="flex-shrink mx-4 text-gray-400 text-sm">OU</span>
          <div className="flex-grow border-t border-gray-600"></div>
        </div>
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
              required
              className="w-full bg-gray-900 border border-gray-600 rounded-lg py-3 pl-10 pr-4 focus:ring-2 focus:ring-red-500 focus:border-red-500 transition-all placeholder-gray-500 text-white"
            />
          </div>
          <div className="relative">
            <Lock className="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-gray-500" />
            <input
              type="password"
              placeholder="Sua senha"
              required
              className="w-full bg-gray-900 border border-gray-600 rounded-lg py-3 pl-10 pr-4 focus:ring-2 focus:ring-red-500 focus:border-red-500 transition-all placeholder-gray-500 text-white"
            />
          </div>
          <button
            type="submit"
            className="w-full flex items-center justify-center gap-2 bg-red-600 hover:bg-red-700 text-white font-bold py-3 px-6 rounded-lg transition-all"
          >
            Entrar
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
