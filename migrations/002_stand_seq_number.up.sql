ALTER TABLE phones
    ADD COLUMN IF NOT EXISTS stand_seq_number SMALLINT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_phones_stand_seq_number
    ON phones(stand_seq_number)
    WHERE stand_seq_number IS NOT NULL;
