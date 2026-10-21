ALTER TABLE subnets
    ADD COLUMN parent_id UUID REFERENCES subnets(id) ON DELETE SET NULL;

CREATE INDEX idx_subnets_parent_id ON subnets(parent_id);

CREATE INDEX idx_ip_addresses_hostname ON ip_addresses (hostname);
CREATE INDEX idx_subnets_name ON subnets (name);
