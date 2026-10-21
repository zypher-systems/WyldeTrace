CREATE TABLE IF NOT EXISTS app_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO app_settings (key, value)
VALUES ('ip_deleted_retention_days', '90')
ON CONFLICT (key) DO NOTHING;

CREATE INDEX IF NOT EXISTS idx_ip_addresses_deleted_at
    ON ip_addresses (deleted_at)
    WHERE deleted_at IS NOT NULL;
