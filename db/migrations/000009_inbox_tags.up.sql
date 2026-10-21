ALTER TABLE subnets
    ADD COLUMN IF NOT EXISTS tags TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS custom_fields JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE INDEX IF NOT EXISTS idx_subnets_tags ON subnets USING GIN (tags);

CREATE TABLE IF NOT EXISTS discovery_findings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subnet_id UUID NOT NULL REFERENCES subnets(id) ON DELETE CASCADE,
    address INET NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('new_alive', 'stale')),
    hostname TEXT NOT NULL DEFAULT '',
    mac_address TEXT NOT NULL DEFAULT '',
    open_ports TEXT NOT NULL DEFAULT '',
    db_ip_id UUID REFERENCES ip_addresses(id) ON DELETE SET NULL,
    db_status TEXT NOT NULL DEFAULT '',
    job_id UUID REFERENCES discovery_jobs(id) ON DELETE SET NULL,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at TIMESTAMPTZ,
    resolve_action TEXT NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_discovery_findings_open
    ON discovery_findings (subnet_id, host(address), kind)
    WHERE resolved_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_discovery_findings_open_list
    ON discovery_findings (resolved_at, last_seen_at DESC);
