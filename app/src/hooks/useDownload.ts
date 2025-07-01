import { useContext } from "react";
import {
  DownloadContext,
  type DownloadContextType,
} from "../contexts/DownloadProvider";

export const useDownloads = (): DownloadContextType => {
  const context = useContext(DownloadContext);
  if (!context) {
    throw new Error("useDownloads deve ser usado dentro de DownloadProvider");
  }
  return context;
};
