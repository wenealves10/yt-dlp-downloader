DROP INDEX IF EXISTS "youtube_accounts_platform_status_active_idx";

DROP INDEX IF EXISTS "youtube_accounts_platform_idx";

ALTER TABLE "youtube_accounts" DROP COLUMN IF EXISTS "platform";
