DROP INDEX IF EXISTS idx_phones_platform_user_id;
ALTER TABLE phones DROP COLUMN IF EXISTS platform_user_id;
