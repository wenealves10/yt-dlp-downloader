-- Tamanho do arquivo entregue ao usuário. Sem esta coluna não há como responder
-- "quanto o storage guarda hoje" nem "quanto já foi baixado no total": o R2
-- cobra por byte armazenado, e varrer o bucket a cada carregamento do painel
-- seria caro e lento.
--
-- Downloads antigos ficam em 0: o arquivo deles já expirou e foi removido do
-- bucket, então contá-los como armazenamento atual seria pior do que não os
-- contar.
ALTER TABLE "downloads" ADD COLUMN "file_size_bytes" BIGINT NOT NULL DEFAULT 0;

-- Exclusão de usuário é lógica. O histórico de downloads referencia users(id) e
-- um DELETE real levaria junto a contabilidade de armazenamento, além de
-- apagar o rastro de quem gerou o quê.
ALTER TABLE "users" ADD COLUMN "deleted_at" TIMESTAMPTZ DEFAULT null;

CREATE INDEX ON "users" ("deleted_at");

-- A busca do painel filtra por nome e e-mail sem diferenciar maiúsculas.
CREATE INDEX ON "users" (lower("full_name"));

CREATE INDEX ON "users" (lower("email"));

-- As séries temporais do dashboard varrem por período; o índice combinado
-- evita ler a tabela inteira a cada troca de intervalo.
CREATE INDEX ON "downloads" ("created_at", "status");
