DROP TABLE IF EXISTS subnet_settings;
DELETE FROM app_settings WHERE key = 'subnet_utilization_alert_percent';
