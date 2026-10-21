ALTER TABLE webhooks
    ADD COLUMN IF NOT EXISTS format TEXT NOT NULL DEFAULT 'json';

ALTER TABLE webhooks
    DROP CONSTRAINT IF EXISTS webhooks_format_check;

ALTER TABLE webhooks
    ADD CONSTRAINT webhooks_format_check
    CHECK (format IN ('json', 'slack', 'teams'));
