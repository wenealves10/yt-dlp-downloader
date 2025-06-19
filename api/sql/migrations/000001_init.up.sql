CREATE SCHEMA "core";

CREATE TYPE "core"."plan_type" AS ENUM (
  'free',
  'premium',
  'enterprise'
);

CREATE TYPE "core"."download_status" AS ENUM (
  'PENDING',
  'PROCESSING',
  'COMPLETED',
  'FAILED',
  'CANCELED',
  'EXPIRED',
  'RETRYING'
);

CREATE TYPE "core"."format_type" AS ENUM (
  'MP3',
  'M4A',
  'MP4',
  'WEBM',
  'BEST',
  'FLAC'
);

CREATE TABLE "users" (
  "id" UUID PRIMARY KEY,
  "full_name" TEXT NOT NULL,
  "photo_url" TEXT DEFAULT null,
  "email" VARCHAR(255) UNIQUE NOT NULL,
  "hashed_password" TEXT NOT NULL,
  "password_changed_at" TIMESTAMPTZ NOT NULL DEFAULT '0001-01-01 00:00:00Z',
  "active" BOOLEAN NOT NULL DEFAULT true,
  "plan" core.plan_type NOT NULL DEFAULT 'free',
  "daily_limit" INT NOT NULL DEFAULT 2,
  "last_login" TIMESTAMPTZ,
  "is_verified" BOOLEAN NOT NULL DEFAULT false,
  "created_at" TIMESTAMPTZ NOT NULL DEFAULT 'now()',
  "updated_at" TIMESTAMPTZ NOT NULL DEFAULT 'now()'
);

CREATE TABLE "downloads" (
  "id" UUID PRIMARY KEY,
  "user_id" UUID NOT NULL,
  "original_url" TEXT NOT NULL,
  "format" core.format_type NOT NULL,
  "status" core.download_status NOT NULL DEFAULT 'PENDING',
  "thumbnail_url" TEXT DEFAULT null,
  "file_url" TEXT DEFAULT null,
  "expires_at" TIMESTAMPTZ DEFAULT null,
  "duration_seconds" INT DEFAULT 0,
  "error_message" TEXT DEFAULT null,
  "created_at" TIMESTAMPTZ NOT NULL DEFAULT 'now()'
);

CREATE UNIQUE INDEX ON "users" ("email");

CREATE INDEX ON "users" ("plan");

CREATE INDEX ON "users" ("created_at");

CREATE INDEX ON "downloads" ("user_id");

CREATE INDEX ON "downloads" ("status");

CREATE INDEX ON "downloads" ("created_at");

CREATE INDEX ON "downloads" ("expires_at");

ALTER TABLE "downloads" ADD FOREIGN KEY ("user_id") REFERENCES "users" ("id");
