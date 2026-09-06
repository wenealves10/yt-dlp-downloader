DROP INDEX IF EXISTS "downloads_created_at_status_idx";

DROP INDEX IF EXISTS "users_lower_idx1";

DROP INDEX IF EXISTS "users_lower_idx";

DROP INDEX IF EXISTS "users_deleted_at_idx";

ALTER TABLE "users" DROP COLUMN IF EXISTS "deleted_at";

ALTER TABLE "downloads" DROP COLUMN IF EXISTS "file_size_bytes";
