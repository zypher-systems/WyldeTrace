# WyldeTrace security & production hardening

## Before `make prod`

1. Copy `.env.example` → `.env` and **replace every secret**.
2. Leave `WYLDTRACE_DEV` unset or `false`.
3. Set a long random `JWT_SECRET` and a strong `ADMIN_PASSWORD` (change again after first login).
4. Change the Postgres password in `DATABASE_URL`; do not expose port `5433` publicly.
5. Set `CORS_ORIGINS` to your SPA origin(s), comma-separated (`*` is rejected when `WYLDTRACE_DEV` is false).
6. Set `SETTINGS_ENCRYPTION_KEY` to a 32-byte key (64 hex chars) so MFA seeds and SMTP/PowerDNS/SNMP secrets are stored AES-GCM encrypted (`wtenc:v1:…`). Existing plaintext rows are rewritten on boot.
7. Terminate TLS at a reverse proxy in front of `:8080`.
8. Prefer `sslmode=require` (or verify-full) for Postgres when the DB is remote.
9. Optionally set `METRICS_TOKEN` and scrape `/metrics` with `Authorization: Bearer <token>` (or `X-Metrics-Token`).

## Session JWT TTL

Local/OIDC session tokens are HS256 JWTs with a **fixed 24-hour TTL** (`internal/config`). There is no sliding refresh cookie: when the token expires the SPA receives **401** and must sign in again. MFA pending tokens last **5 minutes** and cannot call normal API routes.

## Single-instance / HA

Default deploy is **one API process + one Postgres**. For multi-replica API, all replicas must share the same Postgres: the discovery scheduler acquires a **session-scoped** `pg_try_advisory_lock` on a **dedicated pool connection** for the whole tick (lock and unlock run on that connection only). API-triggered `RunNow` uses a per-job advisory lock the same way so two replicas cannot run the same job concurrently. Prefer a single API replica unless you understand webhook fan-out, sweep concurrency, and shared filesystem/network implications. Do not run independent schedulers against separate databases.

## Reverse proxy TLS (example)

Terminate TLS at the proxy; keep WyldeTrace on HTTP inside the trust boundary.

```nginx
server {
  listen 443 ssl http2;
  server_name ipam.example.com;
  ssl_certificate     /etc/ssl/certs/ipam.crt;
  ssl_certificate_key /etc/ssl/private/ipam.key;

  location / {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
  }
}
```

## Prometheus metrics

Set `METRICS_TOKEN` in production and scrape `GET /metrics` with `Authorization: Bearer <token>` or `X-Metrics-Token: <token>`. When the token is unset (lab), the endpoint is open — keep it on a private network or reverse-proxy ACL. Series include request counts/latency, sweep duration, scheduler ticks, and webhook failures.

## Site scope

Non-admin users with **zero** assigned locations see nothing (fail-closed). Admins and API tokens with an empty location list remain global. Sweeps must target an inventory subnet the caller can access.

## Scanner & webhooks

- Sweeps are limited to inventory subnets.
- Outbound URLs require **https** when `WYLDTRACE_DEV` is false (lab may use http). DNS resolution failures fail closed. Webhooks may not target loopback, link-local, RFC1918, or cloud-metadata. Kea/PowerDNS/SNMP may use private (RFC1918) enterprise hosts but still block loopback/link-local/metadata. PowerDNS URL is re-validated on the async sync path.
- Compose grants `NET_RAW` for ICMP; restrict container network policy in shared environments.

## Local MFA (TOTP)

Local password accounts can enroll TOTP under **Settings → Two-factor authentication**. OIDC/SSO users should enforce MFA at the identity provider; WyldeTrace does not challenge SSO logins with local TOTP.

### Break-glass: disable MFA

If an admin loses authenticator access and recovery codes:

1. **Admin API** (another admin session): `POST /api/v1/users/{id}/mfa/disable`
2. **SQL** (database access):

```sql
UPDATE users
SET totp_enabled = false,
    totp_secret = '',
    recovery_codes = '[]'::jsonb,
    updated_at = now()
WHERE email = 'user@example.com';
```

Prefer the admin API when possible so the action is audited.

## SMTP alerts

Optional SMTP settings (`smtp_enabled`, `smtp_host`, `smtp_port`, `smtp_user`, `smtp_pass`, `smtp_from`, `smtp_to`, templates) send util/discovery and reports-summary alerts in parallel with webhooks. Keep `smtp_pass` out of backups shared broadly; rotate if exposed.
