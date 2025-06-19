import { BrowserRouter, Routes, Route } from "react-router-dom";
import PrivateRoute from "./components/auth/PrivateRoute";
import { DownloaderPage } from "./components/downloader/DownloaderPage";
import { SettingsModal } from "./components/settings/SettingsModal";
import { LoginPage } from "./components/auth/LoginPage";
import { RegistrationPage } from "./components/auth/RegistrationPage";
import NotFound from "./components/notfound/NotFound";

export const App = () => {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/register" element={<RegistrationPage />} />
        <Route path="/" element={<LoginPage />} />
        <Route
          path="/dashboard"
          element={
            <PrivateRoute>
              <>
                <DownloaderPage />
                <SettingsModal />
              </>
            </PrivateRoute>
          }
        />
        <Route path="*" element={<NotFound />} />
      </Routes>
    </BrowserRouter>
  );
};
