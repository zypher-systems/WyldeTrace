ALTER TABLE subnets
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

ALTER TABLE subnets DROP CONSTRAINT IF EXISTS subnets_cidr_key;

CREATE UNIQUE INDEX IF NOT EXISTS idx_subnets_cidr_active
    ON subnets (cidr)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_subnets_deleted_at
    ON subnets (deleted_at)
    WHERE deleted_at IS NOT NULL;

INSERT INTO app_settings (key, value)
VALUES
    ('subnet_deleted_retention_days', '90'),
    ('reports_summary_interval_hours', '24')
ON CONFLICT (key) DO NOTHING;
