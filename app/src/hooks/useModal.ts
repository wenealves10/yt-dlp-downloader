import { useContext } from "react";
import { ModalContext } from "../contexts/ModalConfigProvider";

export const useModalConfig = () => {
  const context = useContext(ModalContext);
  if (!context) {
    throw new Error("useModalConfig must be used within ModalConfigProvider");
  }
  return context;
};
