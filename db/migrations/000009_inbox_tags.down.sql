DROP TABLE IF EXISTS discovery_findings;
DROP INDEX IF EXISTS idx_subnets_tags;
ALTER TABLE subnets DROP COLUMN IF EXISTS custom_fields;
ALTER TABLE subnets DROP COLUMN IF EXISTS tags;
