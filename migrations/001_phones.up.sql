CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS phones (
    serial           VARCHAR(64) PRIMARY KEY,
    state            VARCHAR(32) NOT NULL DEFAULT 'new',
    current_step     INTEGER NOT NULL DEFAULT 0,
    last_error       TEXT,
    model            VARCHAR(128),
    android_version  VARCHAR(32),
    screen_res_x     INTEGER,
    screen_res_y     INTEGER,
    current_ip       VARCHAR(64),
    proxy_id         INTEGER,
    wifi_ssid        VARCHAR(128),
    adb_port         INTEGER DEFAULT 5555,
    last_heartbeat   TIMESTAMPTZ,
    heartbeat_count  INTEGER NOT NULL DEFAULT 0,
    recovery_in_progress BOOLEAN NOT NULL DEFAULT false,
    last_error_hash  VARCHAR(128),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    ready_at         TIMESTAMPTZ,
    retired_at       TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS phone_state_log (
    id           BIGSERIAL PRIMARY KEY,
    serial       VARCHAR(64) NOT NULL REFERENCES phones(serial) ON DELETE CASCADE,
    from_state   VARCHAR(32),
    to_state     VARCHAR(32) NOT NULL,
    step         INTEGER NOT NULL DEFAULT 0,
    error        TEXT,
    duration_ms  INTEGER,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_phones_state ON phones(state);
CREATE INDEX IF NOT EXISTS idx_phone_state_log_serial ON phone_state_log(serial);

CREATE TABLE IF NOT EXISTS phone_tasks (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    serial        VARCHAR(64) NOT NULL REFERENCES phones(serial) ON DELETE CASCADE,
    task_type     VARCHAR(32),
    params        JSONB,
    priority      INTEGER NOT NULL DEFAULT 5,
    status        VARCHAR(32) NOT NULL DEFAULT 'queued',
    assigned_at   TIMESTAMPTZ,
    completed_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
