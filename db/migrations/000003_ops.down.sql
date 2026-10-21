ALTER TABLE ip_addresses
    DROP COLUMN IF EXISTS open_ports,
    DROP COLUMN IF EXISTS mac_address,
    DROP COLUMN IF EXISTS last_seen_at,
    DROP COLUMN IF EXISTS reserved_until;

DROP TABLE IF EXISTS audit_logs;
