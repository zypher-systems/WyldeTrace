-- VRFs, first-class VLANs, IP ranges; unique CIDR per VRF.

CREATE TABLE vrfs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    rd TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO vrfs (id, name, description)
VALUES ('00000000-0000-0000-0000-000000000001', 'Default', 'Default VRF')
ON CONFLICT DO NOTHING;

ALTER TABLE subnets
    ADD COLUMN IF NOT EXISTS vrf_id UUID REFERENCES vrfs(id) ON DELETE RESTRICT;

UPDATE subnets
SET vrf_id = '00000000-0000-0000-0000-000000000001'
WHERE vrf_id IS NULL;

ALTER TABLE subnets
    ALTER COLUMN vrf_id SET DEFAULT '00000000-0000-0000-0000-000000000001',
    ALTER COLUMN vrf_id SET NOT NULL;

DROP INDEX IF EXISTS idx_subnets_cidr_active;
CREATE UNIQUE INDEX idx_subnets_vrf_cidr_active
    ON subnets (vrf_id, cidr)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_subnets_vrf_id ON subnets(vrf_id);

CREATE TABLE vlans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vid INTEGER NOT NULL CHECK (vid >= 1 AND vid <= 4094),
    name TEXT NOT NULL DEFAULT '',
    location_id UUID REFERENCES locations(id) ON DELETE SET NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_vlans_location_vid
    ON vlans (COALESCE(location_id, '00000000-0000-0000-0000-000000000000'), vid);

-- Migrate legacy integer VLAN tags into vlans + UUID FK on subnets.
ALTER TABLE subnets RENAME COLUMN vlan_id TO vlan_vid_legacy;

ALTER TABLE subnets
    ADD COLUMN vlan_id UUID REFERENCES vlans(id) ON DELETE SET NULL;

INSERT INTO vlans (vid, name, location_id)
SELECT DISTINCT s.vlan_vid_legacy, 'VLAN ' || s.vlan_vid_legacy::text, s.location_id
FROM subnets s
WHERE s.vlan_vid_legacy IS NOT NULL
  AND NOT EXISTS (
    SELECT 1 FROM vlans v
    WHERE v.vid = s.vlan_vid_legacy
      AND v.location_id IS NOT DISTINCT FROM s.location_id
  );

UPDATE subnets s
SET vlan_id = (
    SELECT v.id FROM vlans v
    WHERE v.vid = s.vlan_vid_legacy
      AND v.location_id IS NOT DISTINCT FROM s.location_id
    LIMIT 1
)
WHERE s.vlan_vid_legacy IS NOT NULL;

ALTER TABLE subnets DROP COLUMN vlan_vid_legacy;

CREATE TABLE ip_ranges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subnet_id UUID NOT NULL REFERENCES subnets(id) ON DELETE CASCADE,
    name TEXT NOT NULL DEFAULT '',
    start_address INET NOT NULL,
    end_address INET NOT NULL,
    role TEXT NOT NULL DEFAULT 'other'
        CHECK (role IN ('dhcp', 'static', 'reserved', 'other')),
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (start_address <= end_address)
);

CREATE INDEX idx_ip_ranges_subnet_id ON ip_ranges(subnet_id);

CREATE OR REPLACE FUNCTION check_ip_range_in_subnet() RETURNS trigger AS $$
DECLARE
    parent_cidr CIDR;
BEGIN
    SELECT cidr INTO parent_cidr FROM subnets WHERE id = NEW.subnet_id;
    IF parent_cidr IS NULL THEN
        RAISE EXCEPTION 'subnet % not found', NEW.subnet_id;
    END IF;
    IF NOT (NEW.start_address <<= parent_cidr AND NEW.end_address <<= parent_cidr) THEN
        RAISE EXCEPTION 'range %-% is not within subnet %', NEW.start_address, NEW.end_address, parent_cidr;
    END IF;
    IF EXISTS (
        SELECT 1 FROM ip_ranges r
        WHERE r.subnet_id = NEW.subnet_id
          AND r.id IS DISTINCT FROM NEW.id
          AND NEW.start_address <= r.end_address
          AND NEW.end_address >= r.start_address
    ) THEN
        RAISE EXCEPTION 'range overlaps an existing range in this subnet';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_ip_range_in_subnet
    BEFORE INSERT OR UPDATE OF start_address, end_address, subnet_id ON ip_ranges
    FOR EACH ROW EXECUTE PROCEDURE check_ip_range_in_subnet();
