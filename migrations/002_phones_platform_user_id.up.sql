ALTER TABLE phones ADD COLUMN IF NOT EXISTS platform_user_id UUID;

CREATE UNIQUE INDEX IF NOT EXISTS idx_phones_platform_user_id
    ON phones (platform_user_id)
    WHERE platform_user_id IS NOT NULL;
