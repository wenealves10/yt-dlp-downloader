-- Papéis de usuário. O painel de contas do YouTube é exclusivo do super admin;
-- 'admin' fica reservado para futuras permissões intermediárias.
CREATE TYPE "core"."user_role" AS ENUM (
  'user',
  'admin',
  'super_admin'
);

ALTER TABLE "users" ADD COLUMN "role" core.user_role NOT NULL DEFAULT 'user';

CREATE INDEX ON "users" ("role");

-- Estados possíveis da sessão de uma conta. O estado do navegador (ligado ou
-- desligado) NÃO é persistido aqui: ele é runtime e vem do serviço de browser,
-- para que uma reinicialização nunca deixe linhas com estado mentiroso.
CREATE TYPE "core"."youtube_account_status" AS ENUM (
  'NOT_CONFIGURED',
  'AWAITING_LOGIN',
  'AUTHENTICATED',
  'REQUIRES_AUTH',
  'DISABLED',
  'ERROR'
);

-- Uma conta do YouTube controlada pelo super admin. A tabela guarda apenas
-- metadados de gerenciamento: nenhuma senha, nenhum cookie e nenhum token de
-- sessão. A sessão vive no perfil persistente do Chrome, referenciado por
-- profile_dir.
CREATE TABLE "youtube_accounts" (
  "id" UUID PRIMARY KEY,
  "label" TEXT NOT NULL,
  "email" VARCHAR(255) DEFAULT null,
  "status" core.youtube_account_status NOT NULL DEFAULT 'NOT_CONFIGURED',
  "profile_dir" TEXT UNIQUE NOT NULL,
  "active" BOOLEAN NOT NULL DEFAULT true,
  "priority" INT NOT NULL DEFAULT 100,
  "last_checked_at" TIMESTAMPTZ DEFAULT null,
  "last_authenticated_at" TIMESTAMPTZ DEFAULT null,
  "last_used_at" TIMESTAMPTZ DEFAULT null,
  "last_error" TEXT DEFAULT null,
  "created_by" UUID DEFAULT null,
  "created_at" TIMESTAMPTZ NOT NULL DEFAULT now(),
  "updated_at" TIMESTAMPTZ NOT NULL DEFAULT now(),
  "deleted_at" TIMESTAMPTZ DEFAULT null
);

CREATE INDEX ON "youtube_accounts" ("status");

CREATE INDEX ON "youtube_accounts" ("active");

CREATE INDEX ON "youtube_accounts" ("last_used_at");

CREATE INDEX ON "youtube_accounts" ("deleted_at");

ALTER TABLE "youtube_accounts" ADD FOREIGN KEY ("created_by") REFERENCES "users" ("id");
