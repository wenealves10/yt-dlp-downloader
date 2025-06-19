import React, { createContext, useState } from "react";

type ModalContextType = {
  isOpen: boolean;
  toggleModal: () => void;
};

export const ModalContext = createContext<ModalContextType | undefined>(
  undefined
);

export const ModalConfigProvider = ({
  children,
}: {
  children: React.ReactNode;
}) => {
  const [isOpen, setIsOpen] = useState(false);

  const toggleModal = () => {
    setIsOpen((prev) => !prev);
  };

  return (
    <ModalContext.Provider value={{ isOpen, toggleModal }}>
      {children}
    </ModalContext.Provider>
  );
};
