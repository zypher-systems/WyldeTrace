CREATE TABLE IF NOT EXISTS custom_field_definitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type TEXT NOT NULL DEFAULT 'subnet' CHECK (entity_type IN ('subnet')),
    key TEXT NOT NULL,
    label TEXT NOT NULL,
    field_type TEXT NOT NULL CHECK (field_type IN ('string', 'number', 'boolean', 'select')),
    required BOOLEAN NOT NULL DEFAULT false,
    options TEXT[] NOT NULL DEFAULT '{}',
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (entity_type, key)
);

CREATE INDEX IF NOT EXISTS idx_custom_field_defs_entity
    ON custom_field_definitions (entity_type, sort_order, key);
