import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "./index.css";
import { App } from "./App.tsx";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AuthProvider } from "./contexts/AuthProvider.tsx";
import { ModalConfigProvider } from "./contexts/ModalConfigProvider.tsx";
import { BrowserRouter } from "react-router-dom";
import { DownloadProvider } from "./contexts/DownloadProvider.tsx";

const queryClient = new QueryClient();

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <BrowserRouter>
      <QueryClientProvider client={queryClient}>
        <AuthProvider>
          <ModalConfigProvider>
            <DownloadProvider>
              <App />
            </DownloadProvider>
          </ModalConfigProvider>
        </AuthProvider>
      </QueryClientProvider>
    </BrowserRouter>
  </StrictMode>
);
