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
import { DashboardPage } from "./components/admin/DashboardPage";
import { UsersPage } from "./components/admin/UsersPage";
import { DownloadsPage } from "./components/admin/DownloadsPage";
import { ProvidersPage } from "./components/admin/ProvidersPage";
import { IntegrationsPage } from "./components/admin/IntegrationsPage";
import { IntegrationDetailPage } from "./components/admin/IntegrationDetailPage";
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
        <Route path="/admin" element={<DashboardPage />} />
        <Route path="/admin/users" element={<UsersPage />} />
        <Route path="/admin/downloads" element={<DownloadsPage />} />
        <Route path="/admin/providers" element={<ProvidersPage />} />
        <Route path="/admin/integrations" element={<IntegrationsPage />} />
        <Route
          path="/admin/integrations/:id"
          element={<IntegrationDetailPage />}
        />
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
