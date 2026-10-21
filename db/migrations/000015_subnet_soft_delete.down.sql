DELETE FROM app_settings
WHERE key IN ('subnet_deleted_retention_days', 'reports_summary_interval_hours', 'reports_summary_last_sent_at');

-- Soft-deleted subnets (and cascaded IPs) must go before restoring unique(cidr).
DELETE FROM subnets WHERE deleted_at IS NOT NULL;

DROP INDEX IF EXISTS idx_subnets_deleted_at;
DROP INDEX IF EXISTS idx_subnets_cidr_active;

ALTER TABLE subnets DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE subnets ADD CONSTRAINT subnets_cidr_key UNIQUE (cidr);
