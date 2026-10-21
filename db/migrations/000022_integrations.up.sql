-- Phase 5 integrations: SNMP enrichment columns + delivery log + settings defaults.

ALTER TABLE ip_addresses
    ADD COLUMN IF NOT EXISTS interface_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS switch_id TEXT NOT NULL DEFAULT '';

ALTER TABLE discovery_findings
    ADD COLUMN IF NOT EXISTS interface_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS switch_id TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS integration_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider TEXT NOT NULL,
    action TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    status_code INTEGER NOT NULL DEFAULT 0,
    error TEXT NOT NULL DEFAULT '',
    dry_run BOOLEAN NOT NULL DEFAULT false,
    request_body JSONB NOT NULL DEFAULT '{}'::jsonb,
    response_body TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_integration_deliveries_provider_created
    ON integration_deliveries (provider, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_integration_deliveries_created
    ON integration_deliveries (created_at DESC);

INSERT INTO app_settings (key, value) VALUES
    ('powerdns_enabled', 'false'),
    ('powerdns_url', ''),
    ('powerdns_api_key', ''),
    ('powerdns_dry_run', 'true'),
    ('powerdns_server', 'localhost'),
    ('kea_enabled', 'false'),
    ('kea_url', ''),
    ('kea_dry_run', 'true'),
    ('snmp_enabled', 'false'),
    ('snmp_community', 'public'),
    ('snmp_version', '2c'),
    ('snmp_timeout_seconds', '5')
ON CONFLICT (key) DO NOTHING;
