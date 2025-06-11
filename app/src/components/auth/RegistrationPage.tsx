import React from "react";
import { Youtube, User, Mail, Lock } from "lucide-react";

interface RegistrationPageProps {
  onRegister: () => void;
  onSwitchToLogin: () => void;
}

export const RegistrationPage: React.FC<RegistrationPageProps> = ({
  onRegister,
  onSwitchToLogin,
}) => (
  <div className="bg-gray-900 min-h-screen flex items-center justify-center p-4 font-sans">
    <div className="w-full max-w-md">
      <header className="text-center mb-8">
        <div className="flex justify-center items-center gap-4">
          <Youtube className="h-12 w-12 text-red-600" />
          <div>
            <h1 className="text-4xl sm:text-5xl font-bold tracking-tight bg-gradient-to-r from-red-500 to-red-700 text-transparent bg-clip-text">
              Crie sua Conta
            </h1>
            <p className="text-gray-400 mt-1">
              Rápido e fácil, comece a usar agora.
            </p>
          </div>
        </div>
      </header>
      <main className="bg-gray-800 p-8 rounded-xl shadow-2xl border border-gray-700">
        <form
          onSubmit={(e) => {
            e.preventDefault();
            onRegister();
          }}
          className="space-y-4"
        >
          <div className="relative">
            <User className="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-gray-500" />
            <input
              type="text"
              placeholder="Nome completo"
              required
              className="w-full bg-gray-900 border border-gray-600 rounded-lg py-3 pl-10 pr-4 focus:ring-2 focus:ring-red-500 focus:border-red-500 transition-all placeholder-gray-500 text-white"
            />
          </div>
          <div className="relative">
            <Mail className="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-gray-500" />
            <input
              type="email"
              placeholder="Seu melhor email"
              required
              className="w-full bg-gray-900 border border-gray-600 rounded-lg py-3 pl-10 pr-4 focus:ring-2 focus:ring-red-500 focus:border-red-500 transition-all placeholder-gray-500 text-white"
            />
          </div>
          <div className="relative">
            <Lock className="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-gray-500" />
            <input
              type="password"
              placeholder="Crie uma senha forte"
              required
              className="w-full bg-gray-900 border border-gray-600 rounded-lg py-3 pl-10 pr-4 focus:ring-2 focus:ring-red-500 focus:border-red-500 transition-all placeholder-gray-500 text-white"
            />
          </div>
          <div className="relative">
            <Lock className="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-gray-500" />
            <input
              type="password"
              placeholder="Confirme sua senha"
              required
              className="w-full bg-gray-900 border border-gray-600 rounded-lg py-3 pl-10 pr-4 focus:ring-2 focus:ring-red-500 focus:border-red-500 transition-all placeholder-gray-500 text-white"
            />
          </div>
          <button
            type="submit"
            className="w-full flex items-center justify-center gap-2 bg-red-600 hover:bg-red-700 text-white font-bold py-3 px-6 rounded-lg transition-all"
          >
            Criar Conta
          </button>
        </form>
        <p className="text-center text-sm text-gray-400 mt-6">
          Já tem uma conta?{" "}
          <button
            onClick={onSwitchToLogin}
            className="font-medium text-red-500 hover:underline bg-transparent border-none cursor-pointer"
          >
            Faça o login
          </button>
        </p>
      </main>
    </div>
  </div>
);
