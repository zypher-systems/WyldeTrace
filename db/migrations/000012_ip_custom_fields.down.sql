ALTER TABLE ip_addresses DROP COLUMN IF EXISTS custom_fields;

ALTER TABLE custom_field_definitions
    DROP CONSTRAINT IF EXISTS custom_field_definitions_entity_type_check;

ALTER TABLE custom_field_definitions
    ADD CONSTRAINT custom_field_definitions_entity_type_check
    CHECK (entity_type IN ('subnet'));
