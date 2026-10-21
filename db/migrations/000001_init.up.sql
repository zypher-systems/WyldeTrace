CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TYPE user_role AS ENUM ('admin', 'editor', 'readonly');
CREATE TYPE ip_status AS ENUM ('available', 'assigned', 'reserved', 'dhcp');

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role user_role NOT NULL DEFAULT 'readonly',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE locations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE device_types (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE subnets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cidr CIDR NOT NULL UNIQUE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    location_id UUID REFERENCES locations(id) ON DELETE SET NULL,
    vlan_id INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_subnets_cidr ON subnets USING GIST (cidr inet_ops);
CREATE INDEX idx_subnets_location_id ON subnets(location_id);

CREATE TABLE ip_addresses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    address INET NOT NULL UNIQUE,
    subnet_id UUID NOT NULL REFERENCES subnets(id) ON DELETE CASCADE,
    hostname TEXT NOT NULL DEFAULT '',
    status ip_status NOT NULL DEFAULT 'available',
    device_type_id UUID REFERENCES device_types(id) ON DELETE SET NULL,
    location_id UUID REFERENCES locations(id) ON DELETE SET NULL,
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Enforce IP containment in parent subnet via trigger (CHECK cannot use subquery reliably across inserts)
CREATE OR REPLACE FUNCTION check_ip_in_subnet() RETURNS trigger AS $$
DECLARE
    parent_cidr CIDR;
BEGIN
    SELECT cidr INTO parent_cidr FROM subnets WHERE id = NEW.subnet_id;
    IF parent_cidr IS NULL THEN
        RAISE EXCEPTION 'subnet % not found', NEW.subnet_id;
    END IF;
    IF NOT (NEW.address << parent_cidr) THEN
        RAISE EXCEPTION 'address % is not within subnet %', NEW.address, parent_cidr;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_ip_in_subnet
    BEFORE INSERT OR UPDATE OF address, subnet_id ON ip_addresses
    FOR EACH ROW EXECUTE PROCEDURE check_ip_in_subnet();

CREATE INDEX idx_ip_addresses_address ON ip_addresses USING GIST (address inet_ops);
CREATE INDEX idx_ip_addresses_subnet_id ON ip_addresses(subnet_id);
CREATE INDEX idx_ip_addresses_status ON ip_addresses(status);
