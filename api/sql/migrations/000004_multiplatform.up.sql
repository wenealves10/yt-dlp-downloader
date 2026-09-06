-- Plataforma e provider são conceitos distintos e por isso são colunas
-- distintas: hoje todas as plataformas são atendidas pelo yt-dlp, e amanhã uma
-- delas pode ganhar provider próprio sem que o histórico perca sentido.
ALTER TABLE "downloads" ADD COLUMN "platform" TEXT NOT NULL DEFAULT 'youtube';
ALTER TABLE "downloads" ADD COLUMN "provider" TEXT DEFAULT null;

-- O seletor de formato é opaco para a aplicação: quem o interpreta é o
-- provider que o emitiu. Guardamos junto o rótulo mostrado ao usuário, porque
-- o significado de "137" depende da versão do provider e do conteúdo.
ALTER TABLE "downloads" ADD COLUMN "format_id" TEXT DEFAULT null;
ALTER TABLE "downloads" ADD COLUMN "quality_label" TEXT DEFAULT null;

-- Instantâneo do progresso, para a tela reabrir no meio de um download em
-- andamento. O tempo real trafega por SSE; isto é só o último estado conhecido.
ALTER TABLE "downloads" ADD COLUMN "progress_percent" NUMERIC(5,2) NOT NULL DEFAULT 0;
ALTER TABLE "downloads" ADD COLUMN "downloaded_bytes" BIGINT NOT NULL DEFAULT 0;
ALTER TABLE "downloads" ADD COLUMN "total_bytes" BIGINT NOT NULL DEFAULT 0;
ALTER TABLE "downloads" ADD COLUMN "speed_bps" BIGINT NOT NULL DEFAULT 0;
ALTER TABLE "downloads" ADD COLUMN "eta_seconds" INT NOT NULL DEFAULT 0;

-- Quanto o download levou de fato, separado de created_at: entre criar e
-- começar pode haver espera na fila.
ALTER TABLE "downloads" ADD COLUMN "started_at" TIMESTAMPTZ DEFAULT null;
ALTER TABLE "downloads" ADD COLUMN "finished_at" TIMESTAMPTZ DEFAULT null;

-- O uploader/autor, que o painel e o histórico mostram.
ALTER TABLE "downloads" ADD COLUMN "uploader" TEXT DEFAULT null;

CREATE INDEX ON "downloads" ("platform");

-- Os downloads que já existem são todos do YouTube feitos pelo yt-dlp; deixar
-- provider nulo neles faria o histórico parecer incompleto.
UPDATE "downloads" SET "provider" = 'yt-dlp' WHERE "provider" IS NULL;
