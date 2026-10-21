INSERT INTO app_settings (key, value)
VALUES ('audit_retention_days', '365')
ON CONFLICT (key) DO NOTHING;
