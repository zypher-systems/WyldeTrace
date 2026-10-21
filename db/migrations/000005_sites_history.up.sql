CREATE TABLE user_locations (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    location_id UUID NOT NULL REFERENCES locations(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, location_id)
);

CREATE INDEX idx_user_locations_location ON user_locations(location_id);

ALTER TABLE ip_addresses ADD COLUMN deleted_at TIMESTAMPTZ;

ALTER TABLE ip_addresses DROP CONSTRAINT IF EXISTS ip_addresses_address_key;
CREATE UNIQUE INDEX ip_addresses_address_alive_uidx
    ON ip_addresses (address)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_ip_addresses_deleted_at ON ip_addresses(deleted_at);

CREATE TABLE ip_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ip_id UUID,
    address INET NOT NULL,
    subnet_id UUID,
    hostname TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT '',
    device_type_id UUID,
    location_id UUID,
    notes TEXT NOT NULL DEFAULT '',
    mac_address TEXT NOT NULL DEFAULT '',
    open_ports TEXT NOT NULL DEFAULT '',
    reserved_until TIMESTAMPTZ,
    last_seen_at TIMESTAMPTZ,
    action TEXT NOT NULL,
    actor_email TEXT NOT NULL DEFAULT '',
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_ip_history_address ON ip_history USING GIST (address inet_ops);
CREATE INDEX idx_ip_history_ip_id ON ip_history(ip_id);
CREATE INDEX idx_ip_history_created_at ON ip_history(created_at DESC);
