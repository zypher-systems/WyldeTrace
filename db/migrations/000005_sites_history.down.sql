DROP TABLE IF EXISTS ip_history;
DROP INDEX IF EXISTS ip_addresses_address_alive_uidx;
DROP INDEX IF EXISTS idx_ip_addresses_deleted_at;
ALTER TABLE ip_addresses DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE ip_addresses ADD CONSTRAINT ip_addresses_address_key UNIQUE (address);
DROP TABLE IF EXISTS user_locations;
