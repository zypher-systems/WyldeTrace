DROP TRIGGER IF EXISTS trg_ip_range_in_subnet ON ip_ranges;
DROP FUNCTION IF EXISTS check_ip_range_in_subnet();
DROP TABLE IF EXISTS ip_ranges;

ALTER TABLE subnets ADD COLUMN vlan_vid_legacy INTEGER;
UPDATE subnets s
SET vlan_vid_legacy = v.vid
FROM vlans v
WHERE s.vlan_id = v.id;
ALTER TABLE subnets DROP COLUMN vlan_id;
ALTER TABLE subnets RENAME COLUMN vlan_vid_legacy TO vlan_id;

DROP TABLE IF EXISTS vlans;

DROP INDEX IF EXISTS idx_subnets_vrf_cidr_active;
ALTER TABLE subnets DROP COLUMN IF EXISTS vrf_id;
CREATE UNIQUE INDEX idx_subnets_cidr_active ON subnets (cidr) WHERE deleted_at IS NULL;

DROP TABLE IF EXISTS vrfs;
