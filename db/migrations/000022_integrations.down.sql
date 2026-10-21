DELETE FROM app_settings WHERE key IN (
    'powerdns_enabled', 'powerdns_url', 'powerdns_api_key', 'powerdns_dry_run', 'powerdns_server',
    'kea_enabled', 'kea_url', 'kea_dry_run',
    'snmp_enabled', 'snmp_community', 'snmp_version', 'snmp_timeout_seconds'
);

DROP TABLE IF EXISTS integration_deliveries;

ALTER TABLE discovery_findings
    DROP COLUMN IF EXISTS interface_name,
    DROP COLUMN IF EXISTS switch_id;

ALTER TABLE ip_addresses
    DROP COLUMN IF EXISTS interface_name,
    DROP COLUMN IF EXISTS switch_id;
