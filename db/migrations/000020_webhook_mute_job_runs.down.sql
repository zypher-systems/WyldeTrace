DROP TABLE IF EXISTS app_notifications;
DROP TABLE IF EXISTS discovery_job_runs;
DROP TABLE IF EXISTS webhook_event_mutes;
DELETE FROM app_settings WHERE key = 'webhook_mute_hours';
