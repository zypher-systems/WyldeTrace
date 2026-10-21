DROP INDEX IF EXISTS idx_subnets_name;
DROP INDEX IF EXISTS idx_ip_addresses_hostname;
DROP INDEX IF EXISTS idx_subnets_parent_id;
ALTER TABLE subnets DROP COLUMN IF EXISTS parent_id;
