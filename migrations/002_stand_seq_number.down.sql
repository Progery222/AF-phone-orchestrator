DROP INDEX IF EXISTS idx_phones_stand_seq_number;

ALTER TABLE phones
    DROP COLUMN IF EXISTS stand_seq_number;
