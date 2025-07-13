import React, { useEffect, useState } from "react";
import { X, User, Mail, KeyRound, Lock } from "lucide-react";
import { useModalConfig } from "../../hooks/useModal";
import { useAuth } from "../../hooks/useAuth";
import { useUpdatePassword, useUpdateProfile } from "../../hooks/useProfile";
import { bucketHost } from "../../constants/config";

export const SettingsModal: React.FC = () => {
  const { user } = useAuth();
  const { isOpen, toggleModal } = useModalConfig();
  const [activeTab, setActiveTab] = useState("profile");
  const [name, setName] = useState(() => user?.full_name || "");
  const [photo, setPhoto] = useState<File | null>(null);
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [error, setError] = useState("");
  const updateProfile = useUpdateProfile();
  const updatePassword = useUpdatePassword();

  const handleSaveProfile = (e: React.FormEvent) => {
    e.preventDefault();
    updateProfile.mutate({ full_name: name, photo });
  };

  useEffect(() => {
    if (updateProfile.isSuccess) {
      toggleModal();
    }
  }, [updateProfile.isSuccess]);

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
              <div className="text-center mb-4 flex flex-col items-center">
                <div
                  className="w-24 h-24 rounded-full overflow-hidden border-2 border-red-500 cursor-pointer relative group"
                  onClick={() => document.getElementById("fotoUpload")?.click()}
                >
                  {photo ? (
                    <img
                      src={URL.createObjectURL(photo)}
                      alt="Preview"
                      className="w-full h-full object-cover"
                    />
                  ) : user?.photo_url ? (
                    <img
                      src={`${bucketHost}/${user.photo_url}`}
                      alt="Avatar"
                      className="w-full h-full object-cover"
                    />
                  ) : (
                    <div className="w-full h-full bg-gray-600 flex items-center justify-center text-white text-3xl">
                      {user?.full_name?.charAt(0).toUpperCase() || "U"}
                    </div>
                  )}
                  <div className="absolute inset-0 bg-black/40 opacity-0 group-hover:opacity-100 flex items-center justify-center text-white text-xs transition">
                    Clique para alterar
                  </div>
                </div>

                <button
                  type="button"
                  onClick={() => document.getElementById("fotoUpload")?.click()}
                  className="mt-2 text-sm text-red-400 hover:underline"
                >
                  Alterar foto
                </button>

                {/* input hidden de upload */}
                <input
                  id="fotoUpload"
                  type="file"
                  accept="image/png, image/jpeg"
                  onChange={(e) => {
                    const file = e.target.files?.[0];
                    if (file) setPhoto(file);
                  }}
                  className="hidden"
                />
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
                disabled={updateProfile.isPending}
              >
                {updateProfile.isPending
                  ? "Atualizando..."
                  : "Salvar Alterações"}
              </button>
            </form>
          )}
          {activeTab === "password" && (
            <form
              onSubmit={(e) => {
                e.preventDefault();
                setError("");

                if (!currentPassword || !newPassword || !confirmPassword) {
                  setError("Preencha todos os campos.");
                  return;
                }

                if (newPassword !== confirmPassword) {
                  setError("As senhas não coincidem.");
                  return;
                }

                updatePassword.mutate(
                  {
                    current_password: currentPassword,
                    new_password: newPassword,
                  },
                  {
                    onSuccess: () => {
                      setCurrentPassword("");
                      setNewPassword("");
                      setConfirmPassword("");
                      toggleModal();
                    },
                    onError: (err: any) => {
                      const msg =
                        err?.response?.data?.message ||
                        err?.message ||
                        "Erro ao alterar senha.";
                      setError(msg);
                    },
                  }
                );
              }}
              className="space-y-4 animate-fade-in-fast"
            >
              <div className="relative">
                <KeyRound className="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-gray-500" />
                <input
                  type="password"
                  value={currentPassword}
                  onChange={(e) => setCurrentPassword(e.target.value)}
                  placeholder="Senha Atual"
                  required
                  className="w-full bg-gray-900 border border-gray-600 rounded-lg py-3 pl-10 pr-4 focus:ring-2 focus:ring-red-500 focus:border-red-500 text-white"
                />
              </div>
              <div className="relative">
                <Lock className="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-gray-500" />
                <input
                  type="password"
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  placeholder="Nova Senha"
                  required
                  className="w-full bg-gray-900 border border-gray-600 rounded-lg py-3 pl-10 pr-4 focus:ring-2 focus:ring-red-500 focus:border-red-500 text-white"
                />
              </div>
              <div className="relative">
                <Lock className="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-gray-500" />
                <input
                  type="password"
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  placeholder="Confirme a Nova Senha"
                  required
                  className="w-full bg-gray-900 border border-gray-600 rounded-lg py-3 pl-10 pr-4 focus:ring-2 focus:ring-red-500 focus:border-red-500 text-white"
                />
              </div>

              {error && (
                <p className="text-sm text-red-400 text-center -mt-2">
                  {error}
                </p>
              )}

              <button
                type="submit"
                className="w-full flex items-center justify-center gap-2 bg-red-600 hover:bg-red-700 text-white font-bold py-2.5 px-6 rounded-lg transition-all"
                disabled={updatePassword.isPending}
              >
                {updatePassword.isPending ? "Alterando..." : "Alterar Senha"}
              </button>
            </form>
          )}
        </div>
      </div>
    </div>
  );
};
