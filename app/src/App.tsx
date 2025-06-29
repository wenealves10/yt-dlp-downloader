import { Routes, Route } from "react-router-dom";
import PrivateRoute from "./components/auth/PrivateRoute";
import { DownloaderPage } from "./components/downloader/DownloaderPage";
import { SettingsModal } from "./components/settings/SettingsModal";
import { LoginPage } from "./components/auth/LoginPage";
import { RegistrationPage } from "./components/auth/RegistrationPage";
import NotFound from "./components/notfound/NotFound";

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
      <Route path="*" element={<NotFound />} />
    </Routes>
  );
};
