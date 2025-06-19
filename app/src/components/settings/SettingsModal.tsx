import React, { useEffect, useState } from "react";
import { X, User, Mail, KeyRound, Lock } from "lucide-react";
import { useModalConfig } from "../../hooks/useModal";
import { useAuth } from "../../hooks/useAuth";

export const SettingsModal: React.FC = () => {
  const { user } = useAuth();
  const { isOpen, toggleModal } = useModalConfig();
  const [activeTab, setActiveTab] = useState("profile");
  const [name, setName] = useState("");

  useEffect(() => {
    if (user) {
      setName(user.full_name || "");
    }
  }, [user]);

  const handleSaveProfile = (e: React.FormEvent) => {
    e.preventDefault();
    toggleModal();
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center z-50 animate-fade-in-fast">
      <div className="bg-gray-800 w-full max-w-lg rounded-xl shadow-2xl border border-gray-700 m-4">
        <header className="p-4 border-b border-gray-700 flex justify-between items-center">
          <h2 className="text-lg font-semibold text-white">
            Configurações da Conta
          </h2>
          <button
            onClick={toggleModal}
            className="text-gray-400 hover:text-white"
          >
            <X />
          </button>
        </header>
        <nav className="p-2 flex gap-2 border-b border-gray-700">
          <button
            onClick={() => setActiveTab("profile")}
            className={`px-4 py-2 text-sm rounded-md transition-colors ${
              activeTab === "profile"
                ? "bg-red-600 text-white"
                : "text-gray-300 hover:bg-gray-700"
            }`}
          >
            Perfil
          </button>
          <button
            onClick={() => setActiveTab("password")}
            className={`px-4 py-2 text-sm rounded-md transition-colors ${
              activeTab === "password"
                ? "bg-red-600 text-white"
                : "text-gray-300 hover:bg-gray-700"
            }`}
          >
            Senha
          </button>
        </nav>
        <div className="p-6">
          {activeTab === "profile" && (
            <form
              onSubmit={handleSaveProfile}
              className="space-y-4 animate-fade-in-fast"
            >
              <div className="text-center mb-4">
                <img
                  src={user?.full_name}
                  alt="Avatar"
                  className="w-24 h-24 rounded-full mx-auto mb-2 border-2 border-red-500"
                />
                <button
                  type="button"
                  className="text-sm text-red-400 hover:underline"
                >
                  Alterar foto
                </button>
              </div>
              <div className="relative">
                <User className="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-gray-500" />
                <input
                  type="text"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="Seu nome"
                  required
                  className="w-full bg-gray-900 border border-gray-600 rounded-lg py-3 pl-10 pr-4 focus:ring-2 focus:ring-red-500 focus:border-red-500 text-white"
                />
              </div>
              <div className="relative">
                <Mail className="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-gray-500" />
                <input
                  type="email"
                  value={user?.email}
                  disabled
                  className="w-full bg-gray-700/50 border border-gray-600 rounded-lg py-3 pl-10 pr-4 text-gray-400 cursor-not-allowed"
                />
              </div>
              <button
                type="submit"
                className="w-full flex items-center justify-center gap-2 bg-red-600 hover:bg-red-700 text-white font-bold py-2.5 px-6 rounded-lg transition-all"
              >
                Salvar Alterações
              </button>
            </form>
          )}
          {activeTab === "password" && (
            <form className="space-y-4 animate-fade-in-fast">
              <div className="relative">
                <KeyRound className="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-gray-500" />
                <input
                  type="password"
                  placeholder="Senha Atual"
                  required
                  className="w-full bg-gray-900 border border-gray-600 rounded-lg py-3 pl-10 pr-4 focus:ring-2 focus:ring-red-500 focus:border-red-500 text-white"
                />
              </div>
              <div className="relative">
                <Lock className="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-gray-500" />
                <input
                  type="password"
                  placeholder="Nova Senha"
                  required
                  className="w-full bg-gray-900 border border-gray-600 rounded-lg py-3 pl-10 pr-4 focus:ring-2 focus:ring-red-500 focus:border-red-500 text-white"
                />
              </div>
              <div className="relative">
                <Lock className="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-gray-500" />
                <input
                  type="password"
                  placeholder="Confirme a Nova Senha"
                  required
                  className="w-full bg-gray-900 border border-gray-600 rounded-lg py-3 pl-10 pr-4 focus:ring-2 focus:ring-red-500 focus:border-red-500 text-white"
                />
              </div>
              <button
                type="submit"
                className="w-full flex items-center justify-center gap-2 bg-red-600 hover:bg-red-700 text-white font-bold py-2.5 px-6 rounded-lg transition-all"
              >
                Alterar Senha
              </button>
            </form>
          )}
        </div>
      </div>
    </div>
  );
};
