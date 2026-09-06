DROP INDEX IF EXISTS "downloads_platform_idx";

ALTER TABLE "downloads"
  DROP COLUMN IF EXISTS "uploader",
  DROP COLUMN IF EXISTS "finished_at",
  DROP COLUMN IF EXISTS "started_at",
  DROP COLUMN IF EXISTS "eta_seconds",
  DROP COLUMN IF EXISTS "speed_bps",
  DROP COLUMN IF EXISTS "total_bytes",
  DROP COLUMN IF EXISTS "downloaded_bytes",
  DROP COLUMN IF EXISTS "progress_percent",
  DROP COLUMN IF EXISTS "quality_label",
  DROP COLUMN IF EXISTS "format_id",
  DROP COLUMN IF EXISTS "provider",
  DROP COLUMN IF EXISTS "platform";
