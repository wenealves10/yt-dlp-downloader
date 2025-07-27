import React, { useState, useEffect, useRef } from "react";
import { bucketHost } from "../../constants/config";

import { Settings, LogOut } from "lucide-react";
import type { User } from "../../interface/User";
import { useModalConfig } from "../../hooks/useModal";

interface UserMenuProps {
  user: User | null;
  onLogout: () => void;
}

export const UserMenu: React.FC<UserMenuProps> = ({ user, onLogout }) => {
  const [isOpen, setIsOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  const { toggleModal } = useModalConfig();

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setIsOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  return (
    <div className="relative" ref={menuRef}>
      <button onClick={() => setIsOpen(!isOpen)}>
        {user?.photo_url && (
          // <img
          //   src={`${bucketHost}/${user.photo_url}`}
          //   alt="Avatar"
          //   className="w-10 h-10 rounded-full border-2 border-gray-600 hover:border-red-500 transition-colors"
          // />

          <img
            className="w-10 h-10 rounded-full border-2 border-gray-600 hover:border-red-500 transition-colors cursor-pointer"
            src={`${bucketHost}/${user.photo_url}`}
            alt="Avatar Profile"
          />
        )}
        {!user?.photo_url && (
          <div className="w-10 h-10 cursor-pointer rounded-full bg-gray-600 flex items-center justify-center text-white">
            {user?.full_name?.charAt(0).toUpperCase() || "U"}
          </div>
        )}
      </button>
      {isOpen && (
        <div className="absolute right-0 mt-2 w-56 bg-gray-800 rounded-lg shadow-xl border border-gray-700 animate-fade-in-fast z-10">
          <div className="p-4 border-b border-gray-700">
            <p className="font-semibold text-white truncate">
              {user?.full_name}
            </p>
            <p className="text-sm text-gray-400 truncate">{user?.email}</p>
          </div>
          <div className="py-1">
            <button
              onClick={() => {
                toggleModal();
                setIsOpen(false);
              }}
              className="w-full text-left flex items-center gap-3 px-4 py-2 text-sm text-gray-300 hover:bg-gray-700 hover:text-white"
            >
              <Settings size={16} />
              Configurações
            </button>
            <button
              onClick={onLogout}
              className="w-full text-left flex items-center gap-3 px-4 py-2 text-sm text-red-400 hover:bg-gray-700 hover:text-red-400"
            >
              <LogOut size={16} />
              Sair
            </button>
          </div>
        </div>
      )}
    </div>
  );
};
