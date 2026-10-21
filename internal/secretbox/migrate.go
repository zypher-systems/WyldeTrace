package secretbox

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

var settingSecretKeys = []string{
	"smtp_pass",
	"powerdns_api_key",
	"snmp_community",
}

// MigratePlaintext encrypts legacy plaintext secrets in-place when a box is configured.
func MigratePlaintext(ctx context.Context, pool *pgxpool.Pool, box *Box) error {
	if box == nil || pool == nil {
		return nil
	}
	for _, key := range settingSecretKeys {
		var value string
		err := pool.QueryRow(ctx, `SELECT value FROM app_settings WHERE key = $1`, key).Scan(&value)
		if err != nil || value == "" || IsEncrypted(value) {
			continue
		}
		sealed, err := box.Seal(value)
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, `UPDATE app_settings SET value = $1, updated_at = now() WHERE key = $2`, sealed, key); err != nil {
			return err
		}
		log.Printf("secretbox: encrypted app_settings.%s", key)
	}
	rows, err := pool.Query(ctx, `SELECT id, totp_secret FROM users WHERE totp_secret <> '' AND totp_secret NOT LIKE 'wtenc:v1:%'`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id any
		var secret string
		if err := rows.Scan(&id, &secret); err != nil {
			return err
		}
		sealed, err := box.Seal(secret)
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, `UPDATE users SET totp_secret = $1, updated_at = now() WHERE id = $2`, sealed, id); err != nil {
			return err
		}
		log.Printf("secretbox: encrypted totp_secret for user %v", id)
	}
	return rows.Err()
}
