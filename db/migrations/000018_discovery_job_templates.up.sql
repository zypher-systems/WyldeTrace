CREATE TABLE IF NOT EXISTS discovery_job_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    interval_minutes INTEGER NOT NULL DEFAULT 60 CHECK (interval_minutes >= 1),
    enrich BOOLEAN NOT NULL DEFAULT true,
    enabled_default BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO discovery_job_templates (name, interval_minutes, enrich, enabled_default)
VALUES
    ('Hourly + enrich', 60, true, true),
    ('Daily light', 1440, false, true),
    ('Every 15m + enrich', 15, true, true)
ON CONFLICT (name) DO NOTHING;
