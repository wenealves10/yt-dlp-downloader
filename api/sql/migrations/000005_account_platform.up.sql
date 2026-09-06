-- Contas gerenciadas deixam de ser exclusivas do YouTube.
--
-- Vimeo recusa até a leitura de metadados sem sessão, e Pinterest, Reddit e X
-- bloqueiam o IP do datacenter enquanto atendem normalmente de uma conexão
-- residencial. Nos dois casos a saída é a mesma que já funcionava para o
-- YouTube: uma sessão autenticada de verdade, criada por login manual no
-- navegador remoto.
--
-- A tabela mantém o nome. Renomeá-la arrastaria queries, tipos gerados e o
-- pacote inteiro sem mudar comportamento nenhum; a coluna abaixo é o que de
-- fato faltava.
ALTER TABLE "youtube_accounts"
  ADD COLUMN "platform" TEXT NOT NULL DEFAULT 'youtube';

-- O padrão cobre as linhas existentes: até esta migração, toda conta era do
-- YouTube.
CREATE INDEX ON "youtube_accounts" ("platform");

-- O rodízio escolhe por plataforma; este índice é o que ele percorre.
CREATE INDEX ON "youtube_accounts" ("platform", "status", "active");
