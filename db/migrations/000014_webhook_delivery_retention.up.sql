INSERT INTO app_settings (key, value)
VALUES ('webhook_delivery_retention_days', '30')
ON CONFLICT (key) DO NOTHING;
