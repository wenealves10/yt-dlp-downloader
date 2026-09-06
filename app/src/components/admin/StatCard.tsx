import React from "react";
import type { LucideIcon } from "lucide-react";

interface Props {
  rotulo: string;
  valor: string;
  detalhe?: string;
  icone?: LucideIcon;
  // destaque tira o card do cinza padrão. Reservado para o número que responde
  // à pergunta principal da tela; usar em todos anula o efeito.
  destaque?: boolean;
}

export const StatCard: React.FC<Props> = ({
  rotulo,
  valor,
  detalhe,
  icone: Icone,
  destaque = false,
}) => (
  <div
    className={`rounded-xl border p-4 ${
      destaque ? "bg-gray-800 border-gray-600" : "bg-gray-800/60 border-gray-700"
    }`}
  >
    <div className="flex items-center justify-between gap-2">
      <p className="text-xs font-medium text-gray-400 uppercase tracking-wide">
        {rotulo}
      </p>
      {Icone && <Icone size={16} className="text-gray-500 shrink-0" />}
    </div>
    <p className="mt-2 text-2xl font-semibold text-gray-50 tabular-nums">{valor}</p>
    {detalhe && <p className="mt-1 text-xs text-gray-500">{detalhe}</p>}
  </div>
);
