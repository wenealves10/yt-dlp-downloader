import { Suspense, lazy } from "react";
import { Routes, Route } from "react-router-dom";
import PrivateRoute from "./components/auth/PrivateRoute";
import { DownloaderPage } from "./components/downloader/DownloaderPage";
import { SettingsModal } from "./components/settings/SettingsModal";
import { LoginPage } from "./components/auth/LoginPage";
import { RegistrationPage } from "./components/auth/RegistrationPage";
import NotFound from "./components/notfound/NotFound";
import SuperAdminRoute from "./components/auth/SuperAdminRoute";
import { YoutubeAccountsPage } from "./components/admin/YoutubeAccountsPage";
import Loading from "./components/loading/Loading";

// O cliente noVNC pesa algumas centenas de kB e só interessa ao super admin.
// Carregá-lo sob demanda mantém o bundle do usuário comum inalterado.
const RemoteBrowserPage = lazy(() =>
  import("./components/admin/RemoteBrowserPage").then((module) => ({
    default: module.RemoteBrowserPage,
  }))
);

export const App = () => {
  return (
    <Routes>
      <Route path="/register" element={<RegistrationPage />} />
      <Route path="/" element={<LoginPage />} />
      <Route element={<PrivateRoute />}>
        <Route
          path="/dashboard"
          element={
            <>
              <DownloaderPage />
              <SettingsModal />
            </>
          }
        />
      </Route>
      {/* Painel do super admin. A guarda aqui é conveniência de navegação: a
          autorização real é feita pela API em cada requisição. */}
      <Route element={<SuperAdminRoute />}>
        <Route
          path="/admin/youtube/accounts"
          element={<YoutubeAccountsPage />}
        />
        <Route
          path="/admin/youtube/accounts/:id/browser"
          element={
            <Suspense fallback={<Loading />}>
              <RemoteBrowserPage />
            </Suspense>
          }
        />
      </Route>
      <Route path="*" element={<NotFound />} />
    </Routes>
  );
};
