-- Remove índices
DROP INDEX IF EXISTS "downloads_expires_at_idx";
DROP INDEX IF EXISTS "downloads_created_at_idx";
DROP INDEX IF EXISTS "downloads_status_idx";
DROP INDEX IF EXISTS "downloads_user_id_idx";

DROP INDEX IF EXISTS "users_created_at_idx";
DROP INDEX IF EXISTS "users_plan_idx";
DROP INDEX IF EXISTS "users_email_idx";

-- Remove tabelas
DROP TABLE IF EXISTS "downloads";
DROP TABLE IF EXISTS "users";

-- Remove tipos ENUM
DROP TYPE IF EXISTS "core"."format_type";
DROP TYPE IF EXISTS "core"."download_status";
DROP TYPE IF EXISTS "core"."plan_type";

-- Remove schema
DROP SCHEMA IF EXISTS "core" CASCADE;
