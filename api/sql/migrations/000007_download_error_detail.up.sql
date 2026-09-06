-- Guarda o motivo TÉCNICO da falha, separado da mensagem que o cliente lê.
--
-- error_message é público: vai para o card do usuário e precisa ser genérico.
-- Sem uma coluna própria para o detalhe, diagnosticar um download que falhou
-- exigia entrar no container e caçar a linha no log — e o histórico, que é
-- justamente onde a falha aparece, não dizia nada a quem opera.
--
-- Esta coluna NUNCA é devolvida a um usuário comum.
ALTER TABLE "downloads"
  ADD COLUMN "error_detail" TEXT DEFAULT null;
