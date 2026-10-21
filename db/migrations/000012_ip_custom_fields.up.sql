ALTER TABLE custom_field_definitions
    DROP CONSTRAINT IF EXISTS custom_field_definitions_entity_type_check;

ALTER TABLE custom_field_definitions
    ADD CONSTRAINT custom_field_definitions_entity_type_check
    CHECK (entity_type IN ('subnet', 'ip'));

ALTER TABLE ip_addresses
    ADD COLUMN IF NOT EXISTS custom_fields JSONB NOT NULL DEFAULT '{}'::jsonb;
