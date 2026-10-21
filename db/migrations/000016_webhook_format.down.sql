ALTER TABLE webhooks DROP CONSTRAINT IF EXISTS webhooks_format_check;
ALTER TABLE webhooks DROP COLUMN IF EXISTS format;
