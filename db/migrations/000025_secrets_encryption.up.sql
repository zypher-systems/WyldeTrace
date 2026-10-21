-- Secrets at rest are envelope-encrypted by the API (AES-GCM, wtenc:v1: prefix)
-- when SETTINGS_ENCRYPTION_KEY is set. No schema change; existing plaintext
-- rows are rewritten on boot via internal/secretbox migrate.
SELECT 1;
