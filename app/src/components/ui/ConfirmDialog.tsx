import React, { useEffect, useRef } from "react";
import { Button, Modal, ModalBody } from "flowbite-react";
import { Loader, TriangleAlert, X } from "lucide-react";

interface ConfirmDialogProps {
  open: boolean;
  /** Pergunta principal. Curta: o detalhe vai em `alvo` e `descricao`. */
  titulo: string;
  /** O que exatamente será afetado — um e-mail, um título de vídeo. */
  alvo?: string;
  /** Consequência da ação, em uma linha. */
  descricao?: React.ReactNode;
  confirmarLabel?: string;
  cancelarLabel?: string;
  /** Enquanto verdadeiro o diálogo fica aberto, travado e com spinner. */
  processando?: boolean;
  erro?: string;
  onConfirm: () => void;
  onCancel: () => void;
}

// Diálogo de confirmação para ações destrutivas, no mesmo estilo do que já
// existe no card de download. Substitui window.confirm, que ignora o tema do
// sistema, mostra o domínio da página no título e não sabe esperar por uma
// requisição.
export const ConfirmDialog: React.FC<ConfirmDialogProps> = ({
  open,
  titulo,
  alvo,
  descricao,
  confirmarLabel = "Sim, tenho certeza",
  cancelarLabel = "Não, cancelar",
  processando = false,
  erro,
  onConfirm,
  onCancel,
}) => {
  const cancelarRef = useRef<HTMLButtonElement>(null);

  // O foco começa em "cancelar", e não em "confirmar": quem aperta Enter por
  // reflexo ao abrir o diálogo não pode apagar nada com isso.
  useEffect(() => {
    if (open && !processando) {
      const id = window.setTimeout(() => cancelarRef.current?.focus(), 50);
      return () => window.clearTimeout(id);
    }
  }, [open, processando]);

  // Fechar durante a requisição deixaria a tela sem resposta enquanto o
  // servidor ainda está trabalhando.
  const fechar = () => {
    if (!processando) onCancel();
  };

  return (
    <Modal show={open} size="md" onClose={fechar} popup dismissible={!processando}>
      <ModalBody className="bg-gray-800 p-6 rounded-md relative">
        <Button
          color="alternative"
          className="absolute top-2 right-2"
          onClick={fechar}
          disabled={processando}
          aria-label="Fechar"
        >
          <X className="h-5 w-5" />
        </Button>

        <div className="text-center">
          <TriangleAlert className="mx-auto mb-4 h-14 w-14 text-gray-400 dark:text-gray-200" />

          <h3 className="mb-2 text-lg font-normal text-gray-500 dark:text-gray-400">
            {titulo}
          </h3>

          {alvo && (
            <p className="mb-2 text-base font-medium text-gray-200 break-words">
              {alvo}
            </p>
          )}

          {descricao && (
            <p className="mb-5 text-sm text-gray-400">{descricao}</p>
          )}

          {erro && (
            <p className="mb-5 bg-red-900/30 border border-red-700 text-red-200 rounded-lg p-3 text-sm text-left">
              {erro}
            </p>
          )}

          <div className="flex justify-center gap-4">
            <Button color="red" onClick={onConfirm} disabled={processando}>
              {processando ? (
                <span className="flex items-center gap-2">
                  <Loader className="animate-spin h-4 w-4" />
                  Removendo...
                </span>
              ) : (
                confirmarLabel
              )}
            </Button>
            <Button
              ref={cancelarRef}
              color="alternative"
              onClick={fechar}
              disabled={processando}
            >
              {cancelarLabel}
            </Button>
          </div>
        </div>
      </ModalBody>
    </Modal>
  );
};
