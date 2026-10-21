-- VRF-scope IP addresses: same address may exist in different VRFs.

ALTER TABLE ip_addresses
    ADD COLUMN IF NOT EXISTS vrf_id UUID REFERENCES vrfs(id) ON DELETE RESTRICT;

UPDATE ip_addresses i
SET vrf_id = s.vrf_id
FROM subnets s
WHERE i.subnet_id = s.id
  AND i.vrf_id IS NULL;

UPDATE ip_addresses
SET vrf_id = '00000000-0000-0000-0000-000000000001'
WHERE vrf_id IS NULL;

ALTER TABLE ip_addresses
    ALTER COLUMN vrf_id SET DEFAULT '00000000-0000-0000-0000-000000000001',
    ALTER COLUMN vrf_id SET NOT NULL;

DROP INDEX IF EXISTS ip_addresses_address_alive_uidx;

CREATE UNIQUE INDEX ip_addresses_vrf_address_alive_uidx
    ON ip_addresses (vrf_id, address)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_ip_addresses_vrf_id ON ip_addresses(vrf_id);
