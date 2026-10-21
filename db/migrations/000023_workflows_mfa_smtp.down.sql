DELETE FROM app_settings WHERE key IN (
    'smtp_enabled',
    'smtp_host',
    'smtp_port',
    'smtp_user',
    'smtp_pass',
    'smtp_from',
    'smtp_to',
    'smtp_subject_template',
    'smtp_body_template'
);

ALTER TABLE users
    DROP COLUMN IF EXISTS recovery_codes,
    DROP COLUMN IF EXISTS totp_enabled,
    DROP COLUMN IF EXISTS totp_secret;

DROP TABLE IF EXISTS ip_requests;
