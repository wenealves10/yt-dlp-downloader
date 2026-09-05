DROP TABLE IF EXISTS "youtube_accounts";

DROP TYPE IF EXISTS "core"."youtube_account_status";

DROP INDEX IF EXISTS "users_role_idx";

ALTER TABLE "users" DROP COLUMN IF EXISTS "role";

DROP TYPE IF EXISTS "core"."user_role";
