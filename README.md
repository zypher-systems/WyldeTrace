# WyldeTrace

WyldeTrace is a lightweight, containerized IP Address Management (IPAM) platform by **Wylde Technologies**.

It provides subnet/IP inventory on PostgreSQL `cidr`/`inet`, JWT RBAC, discovery/scanning, webhooks, and reports — served from a single Go binary with an embedded React SPA.

For end-user workflows, see **[docs/USER_GUIDE.md](docs/USER_GUIDE.md)**. API surface summary: **[docs/openapi.yaml](docs/openapi.yaml)**.

## Host requirements

**Docker Engine + Docker Compose only.** You do not need Go, Node.js, sqlc, or migrate installed on the host. All builds, codegen, tests, and the Vite/Air hot-reload loop run inside Compose services.

## Quick start (development)

```bash
cp .env.example .env
make up
```

| URL | Service |
|-----|---------|
| http://localhost:5173 | Vite SPA (HMR) |
| http://localhost:8080 | API (+ embedded SPA in prod builds) |
| localhost:5433 | Postgres (host-mapped; containers use `db:5432`) |

Default admin (from `.env`): `admin@wyldetech.local` / `changeme`  
`WYLDTRACE_DEV=true` is enabled for local development (optional encryption key, wildcard CORS, http outbound URLs).

## Common commands

```bash
make up            # start db + api (Air) + web (Vite)
make down          # stop stack
make logs          # follow logs
make sqlc          # regenerate sqlc Go code
make migrate       # apply DB migrations
make test          # go test ./... (-tags=integration; needs db via compose)
make prod          # production app + db
```

## CI

Pushes to `main` run the Go tests, the web tests, and a smoke of the production image, then publish that image:

| Image | When |
|-------|------|
| `ghcr.io/zypher-systems/wyldetrace:latest` | every push to `main` |
| `ghcr.io/zypher-systems/wyldetrace:<version>` | the `VERSION` file, on `main` |
| `ghcr.io/zypher-systems/wyldetrace:<commit>` | every publish |

Each tag is a multi-arch manifest (`linux/amd64` and `linux/arm64`).

```bash
docker pull ghcr.io/zypher-systems/wyldetrace:latest
```

## Production

```bash
cp .env.example .env
# require: JWT_SECRET, ADMIN_PASSWORD, DATABASE_URL
# set WYLDTRACE_DEV=false, SETTINGS_ENCRYPTION_KEY, CORS_ORIGINS
# pull the published image, or build it with `make prod`
docker compose -f docker-compose.yml -f docker-compose.prod.yml pull app
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d db app
```

`make prod` still builds from this tree. Set `WYLDTRACE_IMAGE_TAG` to pin a published tag other than `latest`.

Browse http://localhost:8080 — the SPA is embedded in the Go binary. ICMP sweeps may need `NET_RAW` (already set in Compose).

See **[docs/SECURITY.md](docs/SECURITY.md)** for TLS (reverse-proxy example), secrets, webhook SSRF, JWT TTL (24h), metrics, and HA notes. The discovery scheduler uses a Postgres advisory lock so only one API replica runs ticks at a time.

Prometheus scrape: `GET /metrics` (set `METRICS_TOKEN` in production; Bearer / `X-Metrics-Token`).

---

## Features (shipped)

### Auth & access control

| Feature | Notes |
|---------|--------|
| Roles | `admin` · `editor` · `readonly` · `reportview` |
| Site-scoped users | `PUT /api/v1/users/{id}/locations` — empty = **no access** (fail-closed); admins stay global |
| Password change | `POST /api/v1/auth/password` |
| Local MFA (TOTP) | Enroll/confirm/disable under Settings; login challenge + recovery codes; admin break-glass `POST /users/{id}/mfa/disable` |
| Secrets at rest | `SETTINGS_ENCRYPTION_KEY` (64 hex) AES-GCM for MFA seeds + SMTP/PowerDNS/SNMP secrets (`wtenc:v1:…`) |
| OIDC SSO | `OIDC_ENABLED` + issuer/client/redirect; login button; auto-provision |
| OIDC role mapping | `OIDC_ROLE_MAP`, `OIDC_GROUPS_CLAIM`, `OIDC_ROLE_CLAIM`, `OIDC_SYNC_ROLE` |
| OIDC claim preview | Admin: `POST /api/v1/auth/oidc/preview` (Settings UI) |
| Role sync notice | SSO callback redirects with `role_synced` banner on login |
| API tokens | Bearer `wt_…`; role = read/write level; optional site scope; `last_used_at` |
| Token sites | `location_ids` on create; `PUT /api/v1/api-tokens/{id}/locations` |
| Token revoke | `POST /api/v1/api-tokens/{id}/revoke` invalidates immediately |
| Session JWT | 24h TTL; expired/invalid → **401** (SPA re-login) |
| Login rate limit | 8 failed attempts / 15m per client IP |

### Inventory

| Feature | Notes |
|---------|--------|
| Subnets / IPs | CRUD on `/api/v1/subnets`, `/ips`; hierarchy tree + utilization |
| Dense vs sparse | **IPv4-dense** (`/16+`) and **IPv6-dense** (`/112+`): countable util %, next-free walk, maps/sweeps when ≤256 hosts. **Sparse** (e.g. IPv6 `/64`): never enumerate; used-count only; allocate via bounded scan / `start` hint; register-by-address + IP ranges |
| IP status grid | Optional UI at `/subnets/{id}/map` via **Address map**; `GET /ip-map` for IPv4 `/24` / IPv6 `/120` and denser |
| Parent UX | Parent picker on subnet edit; tree-view reparent select |
| Soft-delete IPs | `DELETE /ips/{id}`; restore; Deleted IPs page; retention purge |
| Soft-delete subnets | Same pattern; Deleted subnets page; `subnet_deleted_retention_days` |
| IP history | Timeline page `/ip-history` + per-IP on subnet detail; filterable |
| Util alerts | Global `subnet_utilization_alert_percent` + per-subnet override |
| Tags | Multi-tag filter `?tags=a,b&tag_match=and\|or`; clickable tags |
| Bulk subnets | `POST /api/v1/subnets/bulk` — tags, location, `set_custom_fields` |
| Bulk IPs | `POST /api/v1/ips/bulk` — status, device type, soft-delete, `set_custom_fields` |
| Custom fields | Subnet + IP field defs; toggle CF columns on subnet IP table |
| Device types | Admin CRUD + IP type column |
| Locations | List + **location detail** (`/locations/:id`) with util |
| VRFs | Catalog + per-subnet `vrf_id`; overlapping CIDRs allowed across VRFs; Default VRF protected |
| VLANs | Catalog (VID 1–4094, optional site); subnet `vlan_vid` association |
| CSV export/import | Subnets: tags, location, `custom_fields`; IPs: MAC/ports/enrich + CF |
| DHCP / DNS import | Leases CSV + hosts file |
| PowerDNS write-back | Hostname create/update/delete → A/AAAA + PTR via HTTP API; dry-run + `integration_deliveries` log |
| Kea DHCP sync | `POST /integrations/kea/sync` from `ip_ranges` role=dhcp + reserved/assigned MACs |
| SNMP enrich | `POST /scanner/snmp-enrich` (feature `snmp` or `scanner`); ifTable + ARP (+ optional LLDP) → `interface_name` / `switch_id` |
| SMTP alerts | Settings SMTP; util/discovery + reports-summary email alongside webhooks |
| Allocate child | `POST /subnets/{id}/allocate-child` — first free aligned child prefix |
| Split / merge / resize | Transactional re-CIDR that **moves** IP rows (split soft-deletes parent after creating halves) |
| Search | Global `GET /api/v1/search?q=` — recent queries + single-hit jump |

### Discovery & ops

| Feature | Notes |
|---------|--------|
| ICMP sweep | `POST /scanner/sweep` (+ enrich); dense only (≤256 hosts — IPv4 `/24` / IPv6 `/120`); sparse prefixes register observed hosts |
| Claim / next IP | Claim discovered hosts; allocate next free (`start` hint for sparse) |
| Discovery jobs | Scheduled sweeps; templates; **run history** `GET /discovery-jobs/{id}/runs` |
| Inbox | Claim / dismiss / mark-available; site column + site filter |
| IP requests | Submit / approve / deny / cancel; allocates next free or preferred address |
| Find subnets | Supernet child probing |
| Webhooks | Events + delivery log/retry; formats: `json`, `slack`, `teams`; payload preview |
| Webhook mute | `webhook_mute_hours` suppresses repeat util/discovery webhooks per entity |
| Notifications | Header **Alerts** panel (`GET /api/v1/notifications`); written alongside muted events |
| Reports webhook | `reports.summary` on interval + manual dispatch |
| Audit log | Filters, CSV export, retention purge |
| Backup | JSON export/merge restore with preview counts (admin) |
| Health / metrics | Rich `/healthz` (db + version); `GET /metrics` Prometheus; admin `GET /instance` |

### Dashboard (ops overview)

| Feature | Notes |
|---------|--------|
| UI | `/` home after login (nav **Dashboard**; `/dashboard` + `/reports` redirect here) |
| Getting started | Checklist for admin/editor until dismissed (password → site → subnet → discover → inbox) |
| Action tiles | Subnets · Inbox · Jobs shortcuts with live counts |
| Summary JSON/CSV | Inventory, util top, by-site, discovery, inbox |
| Charts | Status donut + utilization bars |
| Site scope | Honors user/token location memberships |
| Site filter | `?location_id=` on summary + CSV (bookmarkable) |
| Saved views | Browser localStorage named filters; share-link copy; last-exported hint |
| `reportview` role | Dashboard + IP Requests + Settings (password / MFA) |

### UX chrome

| Feature | Notes |
|---------|--------|
| Brand | Logos in `logos/` → `web/public/logos/`; muted grey + teal tokens |
| Pill actions | Compact Edit / Delete (and similar) row buttons |
| Confirm dialog | Centered modal replaces browser `confirm()` for deletes/purges |
| Context menus | Right-click on Subnets, IPs, Inbox, Discovery jobs |
| Bootstrap password | `using_default_password` on `/auth/me`; banner until changed |
| Lab login prefill | Bootstrap creds only on localhost / `.local` hosts |
| Sample seed | Subnets tagged `lab` show a Sample badge |

### Settings retention keys

| Key | Default | Purpose |
|-----|---------|---------|
| `ip_deleted_retention_days` | 90 | Soft-deleted IP purge |
| `subnet_deleted_retention_days` | 90 | Soft-deleted subnet purge |
| `audit_retention_days` | 365 | Audit log purge |
| `webhook_delivery_retention_days` | 30 | Delivery log purge |
| `reports_summary_interval_hours` | 24 | Scheduled `reports.summary` (0 = off) |
| `subnet_utilization_alert_percent` | 90 | Global util webhook threshold (0 = off) |
| `webhook_mute_hours` | 6 | Suppress repeat discovery/util webhooks (0 = off) |
| `powerdns_*` / `kea_*` / `snmp_*` | off / dry-run | Integration URLs, keys, dry-run flags (Settings → Integrations) |

---

## API overview

Auth & identity:

- `POST /api/v1/auth/login`, `GET /auth/me` (includes `using_default_password` when still on bootstrap), `POST /auth/password`
- `GET /auth/oidc/status|login|callback`, `POST /auth/oidc/preview` (admin)

Core inventory:

- CRUD `/locations`, `/device-types`, `/subnets`, `/ips`
- `GET /subnets/tree`, `/subnets/tags`, `/subnets/{id}/utilization|children|ips|ip-map`
- `POST /subnets/{id}/allocate-child|split|merge|resize`
- `POST /subnets/bulk`, `POST /ips/bulk`
- `GET /subnets/deleted`, `POST /subnets/{id}/restore`, `POST /subnets/purge-deleted`
- `GET /ips/deleted`, `POST /ips/{id}/restore`, `POST /ips/purge-deleted`

Import / export:

- `GET /export/subnets.csv` — columns: `cidr,name,description,vlan_id,location_id,location,parent_id,tags,custom_fields`
- `POST /import/subnets.csv` — create or update by CIDR; tags + location + merge `custom_fields` JSON
- `GET /export/ips.csv` — `address,subnet_id,subnet_cidr,hostname,status,notes,location,device_type,mac_address,open_ports,custom_fields`
- `POST /import/ips.csv` — resolve subnet by id/cidr; location/device type by name; merge custom fields JSON
- `POST /import/dhcp-leases.csv`, `POST /import/dns-hosts`

Discovery / automation:

- `POST /scanner/sweep|claim|discover-subnets|/commit|/snmp-enrich`
- `POST /integrations/kea/sync`, `GET /integrations/deliveries`
- CRUD `/discovery-jobs`, templates (`/discovery-jobs/templates`), `POST /discovery-jobs/from-template`
- `GET /discovery-jobs/{id}/runs` — last N run summaries
- `/inbox` (+ `/inbox/bulk`)
- CRUD `/webhooks` (`format`: json|slack|teams), `POST /preview` + `/{id}/preview`, deliveries, retry, purge
- CRUD `/api-tokens` (+ `PUT /{id}/locations`)
- `GET /notifications`, `POST /notifications/read-all`, `POST /notifications/{id}/read`

Reports / admin:

- `GET /reports/summary[?location_id=]`, `/reports/summary.csv`, `POST /reports/summary/dispatch`
- `GET|PUT /settings`, audit logs, `GET /instance` (admin)
- `GET /backup`, `POST /backup/restore/preview`, `POST /backup/restore`
- `GET /ip-history` (paginated filters) · `GET|PUT /subnets/{id}/settings`
- `GET /healthz`, `GET /api/v1/healthz` — status, version, db ping
- `GET /metrics` — Prometheus (request counts, sweep duration, scheduler ticks, webhook failures)

OpenAPI summary: [docs/openapi.yaml](docs/openapi.yaml).

### Webhook events

`discovery.new_alive`, `discovery.stale`, `discovery.error`, `ip.deleted`, `ip.claimed`, `subnet.utilization_high`, `reports.summary`

Delivery envelope (format=`json`):

```json
{ "event": "…", "timestamp": "…", "payload": { } }
```

HMAC header: `X-WyldeTrace-Signature: sha256=…` when a secret is set.

### Backup notes

- **JSON backup** (Settings → Backup): portable inventory export/merge restore.
- **Full Postgres dump**:

```bash
docker compose exec -T db pg_dump -U wyldetrace wyldetrace > backup.sql
docker compose exec -T db psql -U wyldetrace wyldetrace < backup.sql
```

## Branding

Source logos live in [`logos/`](logos/) and are copied into the SPA at `web/public/logos/` (`logo.svg` mark, `logo-full.svg` wordmark). UI tokens: muted grey surfaces (`#141416` / `#1c1c1f`) with teal brand `#2dd4bf` / `#14b8a6`.

## Layout

```
cmd/wyldetrace/     # application entrypoint
internal/           # auth, db, handlers, kea, powerdns, snmp, scanner, server, webhook
db/migrations/      # postgres migrations
db/queries/         # sqlc queries
web/                # React + Vite + Tailwind + TanStack Table
docs/               # user guide and operator docs
```

## License

[MIT](LICENSE). Copyright (c) 2026 Wylde Technologies.
