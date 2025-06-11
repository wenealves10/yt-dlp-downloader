import { useState, type SetStateAction } from "react";
import { LoginPage } from "./components/auth/LoginPage";
import { RegistrationPage } from "./components/auth/RegistrationPage";
import { DownloaderPage } from "./components/downloader/DownloaderPage";
import { SettingsModal } from "./components/settings/SettingsModal";

function App() {
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [currentView, setCurrentView] = useState("login"); // 'login' or 'register'
  const [isSettingsOpen, setIsSettingsOpen] = useState(false);
  const [user, setUser] = useState({
    name: "Usuário de Teste",
    email: "usuario@exemplo.com",
    avatar: "https://placehold.co/100x100/1f2937/FFFFFF?text=U",
  });

  const handleAuthSuccess = () => setIsAuthenticated(true);
  const handleLogout = () => setIsAuthenticated(false);
  const handleUpdateUser = (
    updatedUser: SetStateAction<{ name: string; email: string; avatar: string }>
  ) => setUser(updatedUser);

  if (!isAuthenticated) {
    if (currentView === "login") {
      return (
        <LoginPage
          onLogin={handleAuthSuccess}
          onSwitchToRegister={() => setCurrentView("register")}
        />
      );
    } else {
      return (
        <RegistrationPage
          onRegister={handleAuthSuccess}
          onSwitchToLogin={() => setCurrentView("login")}
        />
      );
    }
  }

  return (
    <>
      <DownloaderPage
        user={user}
        onLogout={handleLogout}
        onOpenSettings={() => setIsSettingsOpen(true)}
      />
      <SettingsModal
        isOpen={isSettingsOpen}
        onClose={() => setIsSettingsOpen(false)}
        user={user}
        onUpdateUser={handleUpdateUser}
      />
    </>
  );
}

export default App;
