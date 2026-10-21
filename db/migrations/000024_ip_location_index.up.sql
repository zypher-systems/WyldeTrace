-- Phase 8: index location-scoped IP lookups.
CREATE INDEX IF NOT EXISTS idx_ip_addresses_location_id
    ON ip_addresses (location_id)
    WHERE location_id IS NOT NULL;
