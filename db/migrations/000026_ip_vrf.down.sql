DROP INDEX IF EXISTS idx_ip_addresses_vrf_id;
DROP INDEX IF EXISTS ip_addresses_vrf_address_alive_uidx;

CREATE UNIQUE INDEX ip_addresses_address_alive_uidx
    ON ip_addresses (address)
    WHERE deleted_at IS NULL;

ALTER TABLE ip_addresses DROP COLUMN IF EXISTS vrf_id;
