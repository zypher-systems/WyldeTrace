-- Phase 6: IP request queue, TOTP MFA columns, SMTP settings.

CREATE TABLE IF NOT EXISTS ip_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    requester_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subnet_id UUID NOT NULL REFERENCES subnets(id) ON DELETE CASCADE,
    range_id UUID REFERENCES ip_ranges(id) ON DELETE SET NULL,
    hostname TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    requested_address INET,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'approved', 'denied')),
    allocated_ip_id UUID REFERENCES ip_addresses(id) ON DELETE SET NULL,
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    review_notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    reviewed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_ip_requests_status_created
    ON ip_requests (status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ip_requests_requester
    ON ip_requests (requester_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ip_requests_subnet
    ON ip_requests (subnet_id);

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS totp_secret TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS totp_enabled BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS recovery_codes JSONB NOT NULL DEFAULT '[]'::jsonb;

INSERT INTO app_settings (key, value) VALUES
    ('smtp_enabled', 'false'),
    ('smtp_host', ''),
    ('smtp_port', '587'),
    ('smtp_user', ''),
    ('smtp_pass', ''),
    ('smtp_from', ''),
    ('smtp_to', ''),
    ('smtp_subject_template', '[WyldeTrace] {{.Title}}'),
    ('smtp_body_template', E'{{.Title}}\n\n{{.Body}}\n\nEvent: {{.Event}}\nEntity: {{.EntityKey}}\n')
ON CONFLICT (key) DO NOTHING;
