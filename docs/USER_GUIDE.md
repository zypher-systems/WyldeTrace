# WyldeTrace User Guide

This guide covers day-to-day use of the WyldeTrace UI. For install, env vars, and API reference, see the [README](../README.md). Hardening and break-glass details: [SECURITY.md](SECURITY.md).

> Status: living document — updated as features ship. Last aligned with VRFs/VLANs, IP requests, MFA, integrations (PowerDNS/Kea/SNMP/SMTP), Dashboard home, Getting started checklist, teal/grey branding, pill actions, confirm modals, and context menus.

---

## 1. Getting started

1. Open the app (dev: http://localhost:5173).
2. Sign in with email/password, or **Sign in with SSO** when OIDC is enabled. Local accounts with MFA enabled will be prompted for a TOTP (or recovery) code after password.
3. Land on **Dashboard** — follow the **Getting started** checklist (change password → site → subnet → discovery → inbox).
   - **Admins** can rename HQ or create locations on the site step.
   - **Editors** cannot create sites; confirm the location an admin assigned you (or ask for membership if Locations looks empty).
4. Your **role** controls what you see:
   - **admin** — full access, users, tokens, webhooks, settings, integrations
   - **editor** — inventory + discovery write (including IP request approve/deny)
   - **readonly** — inventory read; can submit IP requests
   - **reportview** — Dashboard, IP **Requests**, and Settings (password / MFA) only

If your account has **site memberships**, you only see subnets/IPs for those locations. Non-admin users with **no** assigned sites see nothing (**fail-closed**). Admins and API tokens with no site list remain global.

Sessions use a JWT that expires after **24 hours**; after expiry the UI treats API calls as logged out (401) and you sign in again.

Lab hosts prefill bootstrap credentials; production hosts do not. A banner appears until the bootstrap password is changed. Seed subnets tagged `lab` show a **Sample** badge.

---

## 2. Navigation

The left sidebar groups (items depend on role):

| Group | Pages |
|-------|--------|
| Overview | Dashboard, Search, IP history (`reportview`: Dashboard, Requests) |
| Inventory | Subnets, Locations, VRFs, VLANs, Devices; editors also see Deleted IPs / Deleted subnets |
| Discovery | Inbox, Requests, Jobs; editors also see Find subnets |
| Admin | Users, Tokens, Webhooks, Fields, Audit (admins only) |
| Account | Settings |

Mobile: use **Menu** in the top bar. Inbox and Requests show pending counts when non-zero. Header **Alerts** opens the in-app notification panel (API `/api/v1/notifications` — there is no dedicated `/notifications` page).

### Common UI patterns

- **Pill buttons** for row actions (Edit, Delete, History, etc.).
- **Confirm dialogs** (centered) for deletes, purges, and restore — not the browser confirm box.
- **Right-click** a Subnet, IP, Inbox finding, or Discovery job row for a context menu (same actions, faster path).
- Destructive confirms require an explicit click (Enter does not confirm deletes).

---

## 3. Subnets

### Dense IPv4 vs sparse IPv6

WyldeTrace does **not** enumerate IPv6 `/64` (or other huge prefixes). Behavior splits on prefix size:

| Mode | Prefixes | Utilization | Address map / ICMP sweep | Allocate next |
|------|----------|-------------|--------------------------|---------------|
| **Dense** | IPv4 `/16` and longer; IPv6 `/112` and longer | Used / total / free / **percent** | Grid + full sweep when ≤256 hosts (IPv4 `/24`, IPv6 `/120`) | Walks usable hosts |
| **Sparse** | Larger than above (typical IPv6 `/64`) | **Used count only** (no %) | Not supported — message points you to children / register | Bounded scan from network (or optional `start` hint); never walks the whole `/64` |

For sparse subnets: **register addresses by hand** (add IP), use **IP ranges** as pools, and claim **observed** hosts from discovery — do not expect a full occupancy grid.

### List & filter

- Switch **Table** / **Tree** views.
- Utilization shows as a tinted bar/chip per row (and on tree rows). Sparse prefixes show a used count instead of a percent bar.
- Filter by text, or click **tags** (toggle multiple). Use **AND** / **OR** when more than one tag is selected.
- Shareable URL: `/subnets?tags=prod,edge&tag_match=and`.
- In **Tree** view, editors can reparent a subnet with the inline parent select.
- Columns / create form include optional **VRF** and **VLAN** (VID).

### Create & edit

- Editors can add a subnet (CIDR, name, tags, optional parent, VRF, VLAN VID, custom fields).
- Open a subnet for utilization, children, and IPs. Edit includes an explicit **Parent subnet** picker plus VRF/VLAN.
- Soft-delete removes it from inventory; restore from **Deleted subnets**.

### IP status grid (optional)

On a subnet page, open **Address map** for a full-view occupancy grid (`/subnets/:id/map`, IPv4 **/24** / IPv6 **/120** and denser):

- Each cell is labeled with the **last octet** (20 per row); gaps separate cells
- Colors: available / assigned / reserved / dhcp / soft-deleted / open / network-broadcast
- Hover shows address + client hostname; click opens an add/edit popover on the map
- Toggles: **Hide unused** / **Hide used** (or show All)
- Larger / sparse prefixes show used count (and child links when present) instead of a dense grid
- Optional — the subnet detail page stays table-first for everyone else

### Bulk actions

1. Select rows with checkboxes.
2. **Apply tags** (add / remove / replace), **Assign location**, or **Set field** (custom field key + value).
3. Clear selection when done.

### Import / export

- **Export → Subnets CSV** includes `tags`, `location_id`, `location` (name), and `custom_fields` (JSON).
- **Import → Subnets CSV** creates new CIDRs or **updates** existing ones by CIDR.
  - Tags: comma-separated (`prod,edge`)
  - Location: UUID (`location_id`) or exact site name (`location`)
  - Custom fields: JSON object merged onto existing values (omit column to leave unchanged)

---

## 4. IP addresses

Open a subnet → optional **Address map** + IP table.

- **Edit subnet** (header) opens a popup for name, site, VRF, VLAN, tags, parent, and custom fields — the form is not always on the page.
- Editors see a **Next: discover hosts** strip (Discover & enrich → Claim → Inbox). Dense sweeps only; on sparse IPv6, add/claim specific addresses instead of sweeping the `/64`.
- **Add IP** / row **Edit** open a popup for hostname, status, device type, MAC, ports, notes, and custom fields (or use the row context menu).
- **Allocate next** finds the next free among known used addresses (optional `start` cursor for sparse pools).
- Toggle **custom field columns** above the table to pin CF keys as extra columns.
- Soft-delete / restore via **Deleted IPs**.
- **Bulk** (checkboxes): set status, device type, custom field, or soft-delete many at once.

Statuses: `available`, `assigned`, `reserved`, `dhcp`.

### IP history

**Overview → IP history** lists recent IP events (create/update/delete/claim). Filter by address, action, actor, and date range — same style as Audit.

Per-IP history is still available from a subnet’s IP row.

### Import / export (IPs)

- **Export → IPs CSV** includes `subnet_cidr`, `location`, `device_type`, `mac_address`, `open_ports`, and `custom_fields` (JSON) for round-trips.
- **Import → IPs CSV** matches subnet by `subnet_id` or `subnet_cidr`; location and device type by name; merges `custom_fields` JSON onto existing values.

---

## 5. Locations & device types

- **Locations**: sites used for scoping and reporting. Click a name for detail (subnets + utilization).
- **Devices**: device-type catalog used on IP records.

Site membership is fail-closed for non-admins: empty memberships mean no inventory visibility. Assign locations under **Admin → Users** before handing accounts to editors/readonly users.

---

## 6. VRFs

**Inventory → VRFs** lists routing domains so the same CIDR can exist in more than one VRF.

- Everyone with inventory access can **view** VRFs.
- **Admins** create, edit, and delete (the built-in Default VRF cannot be deleted).
- Fields: name, optional route distinguisher (RD, e.g. `65000:1`), description.
- Assign a VRF when creating or editing a subnet; IPs inherit the subnet’s VRF. Overlapping CIDRs are allowed across different VRFs, not within the same one.

---

## 7. VLANs

**Inventory → VLANs** is the VLAN catalog (VID 1–4094).

- **Editors and admins** create, edit, and delete.
- Optional **site** (location) and description.
- On a subnet, set **VLAN VID** to associate the prefix with a VLAN id (shown on the subnet detail header).

Use VLANs for documentation and inventory correlation; they do not push switch config by themselves.

---

## 8. Discovery

| Page | Purpose |
|------|---------|
| Inbox | New alive / stale findings — claim, dismiss, mark available (incl. bulk) |
| Requests | IP address request / approval workflow (see below) |
| Jobs | Scheduled ICMP sweeps; optional **templates**; **History** shows recent run summaries |
| Find subnets | Probe a supernet for unknown child prefixes (editors+) |

Header **Alerts** opens the in-app notification center (discovery/util events). Mark read individually or all at once.

### Inbox tips

- Filter by **kind**, **subnet**, and **site** (site column shows location scope).
- Keyboard (editors): `j`/`k` move · `space` select · `c` claim · `a` mark available · `d` dismiss.

### Job templates

On **Discovery → Jobs**:

1. Pick a template (or Custom) when creating a job, choose a subnet, and create.
2. Admins/editors can save new templates (name, interval, enrich) and delete unused ones.
3. Seeded presets: hourly+enrich, daily light, every 15m+enrich.
4. **Edit** on a job row loads name/subnet/interval/enrich/enabled into the form — save without delete+recreate.
5. **Run now** finishes with a toast (new/stale counts) and an **Open inbox** action filtered to that job’s subnet.

---

## 9. IP requests

**Discovery → Requests** (also under Overview for `reportview`).

Request an address; editors approve and allocate from inventory.

1. **Submit**: choose a subnet (or paste subnet UUID as `reportview`), hostname, optional preferred address, notes.
2. Leave address blank to allocate the next free address on approve.
3. **Editors/admins** filter pending queues, then **Approve** or **Deny**.
4. Requesters (or editors) can **Cancel** a pending request they own.
5. Sidebar badge shows pending count when non-zero.

Approved requests create/update the IP in inventory (allocated address appears on the row). Site scope still applies — you only request against subnets you can see.

---

## 10. Dashboard

Home after login (ops overview — not formal report documents):

- **Getting started** checklist for admins/editors (dismissible).
- **Action tiles**: Subnets, Inbox, Jobs with live counts.
- Rollups: inventory counts, utilization top, by-site, discovery, inbox.
- **charts**: IP status breakdown (donut) and top/by-site utilization bars.
- **Site filter** narrows the summary (`?location_id=` in the URL — bookmarkable).
- **Save view** stores named filters in your browser (localStorage).
- **Copy share link** / **Export CSV**; admins can **Send to webhooks** (`reports.summary`).

Route: `/`. `/dashboard` and `/reports` redirect here. `reportview` users land here; they can open **Requests** and Settings, but not inventory links.

---

## 11. Webhooks (admin)

1. Create a webhook with URL, events, and **format**:
   - `json` — WyldeTrace envelope
   - `slack` — Block Kit–style payload
   - `teams` — Office 365 MessageCard
2. Optional HMAC secret → `X-WyldeTrace-Signature`.
3. Use **Payload preview** (dry-run sample for an event/format) before **Test**.
4. Check the **Delivery log** (retry failed deliveries).

Useful events: discovery.\*, `ip.deleted`, `ip.claimed`, `subnet.utilization_high`, `reports.summary`.

Scheduled summary interval is under **Settings → Reports summary webhooks**.

---

## 12. API tokens (admin)

- Role = capability: `readonly` / `reportview` (read) vs `editor` / `admin` (write).
- Optional **sites**: leave empty for all sites; select locations to site-scope the token.
- Admin tokens are always global (cannot combine with site list).
- Optional expiry in days.
- **Last used** updates on authenticated API calls; Tokens page sorts by recent use.
- Copy the token once at creation — it is not shown again.
- **Revoke** from the Tokens page (or `POST /api/v1/api-tokens/{id}/revoke`) to invalidate immediately; **Delete** removes the row after revoke/expiry cleanup.

Use as: `Authorization: Bearer wt_…`

---

## 13. SSO / OIDC (operators)

Configured via env (see `.env.example`). In **Settings**:

- View issuer, role map, sync flags.
- Admins can **Claim preview** sample groups/role claims to see resolved role and whether sync would change a user.

When `OIDC_SYNC_ROLE` updates a user’s role at login, the login page shows a short notice.

OIDC/SSO users should rely on IdP MFA; WyldeTrace does not challenge SSO logins with local TOTP.

---

## 14. MFA (local accounts)

**Settings → Two-factor authentication** (local password accounts).

### Enroll

1. **Begin enrollment** — copy the secret (or otpauth URL) into your authenticator app.
2. Enter a current TOTP code → **Confirm**.
3. Store the one-time **recovery codes** offline; they are shown once.

### Sign-in

After password, the login page asks for a TOTP code. Use **Use a recovery code** if the authenticator is unavailable (each code is single-use).

### Disable (self-service)

Provide current password + a valid TOTP code under Settings to turn MFA off.

### Break-glass (operators)

If an admin loses both authenticator and recovery codes, another admin can disable MFA via `POST /api/v1/users/{id}/mfa/disable`, or use the SQL path in [SECURITY.md](SECURITY.md). Prefer the API so the action is audited. Keep a second admin (or local break-glass account) before enforcing MFA broadly.

---

## 15. Integrations (admin)

**Settings → Integrations** (and **SMTP alerts** below it). Delivery attempts are logged at `/api/v1/integrations/deliveries`.

| Integration | What it does | UI actions |
|-------------|--------------|------------|
| **PowerDNS** | Hostname create/update/delete → A/AAAA + PTR via HTTP API | Enable, API URL, API key, server ID, dry-run (log only) |
| **Kea DHCP** | Sync reservations from `ip_ranges` (role=dhcp) + reserved/assigned MACs | Enable, Control Agent URL, dry-run, **Sync now** |
| **SNMP** | Switch enrich (ifTable + ARP, optional LLDP) → `interface_name` / `switch_id` | Enable, community, version, timeout, target + **Enrich now** |
| **SMTP** | Email util/discovery and reports-summary alerts alongside webhooks | Host/port/user/pass/from/to + subject/body templates |

Outbound URLs: production (`WYLDTRACE_DEV` unset or false) expects **https** for webhooks; Kea/PowerDNS/SNMP may target private enterprise hosts but still block loopback/link-local/metadata. Prefer **dry-run** until deliveries look right.

Secrets (PowerDNS API key, SNMP community, SMTP password, MFA seeds) are stored encrypted at rest when `SETTINGS_ENCRYPTION_KEY` is set — see Admin runbook.

---

## 16. Settings & retention

Admins configure purge windows for deleted IPs, deleted subnets, audit logs, and webhook deliveries (0 disables automatic purge). Manual **Purge now** requires confirmation.

Also: **webhook mute window** (hours to suppress repeat discovery/util webhooks; 0 = off), utilization alert threshold, password change, MFA, OIDC status, Integrations + SMTP, JSON backup / restore, and an **instance** panel (version, migration, db, login rate limit).

---

## 17. Custom fields

**Fields** admin page defines subnet and/or IP fields (text, number, select, etc.). Values appear on create/edit forms and in the IP table Fields column.

Bulk fill: select subnets (list) or IPs (subnet detail) → choose field + value → **Set field**. Values merge into existing custom-field JSON (other keys unchanged).

---

## 18. Admin runbook

### First boot

1. Copy `.env.example` → `.env`. Set `JWT_SECRET` and Postgres credentials. Leave `WYLDTRACE_DEV=true` for a lab; set it to `false` for production.
2. For production, set `SETTINGS_ENCRYPTION_KEY` to a 32-byte key (**64 hex chars**) so MFA seeds and SMTP/PowerDNS/SNMP secrets are stored AES-GCM encrypted (`wtenc:v1:…`). Required when `WYLDTRACE_DEV` is false; existing plaintext rows are rewritten on boot.
3. `make up` (or your compose stack). Migrations apply on API start.
4. Sign in as bootstrap admin (`admin@wyldetech.local` / `changeme` in lab) and change the password under **Settings**. Optionally enroll MFA before sharing the instance.
5. Cold-lab walkthrough (acceptance): Dashboard checklist → open Lab LAN `/24` → Discover & enrich → claim/dismiss Inbox → confirm Dashboard counts update.
6. Smoke: `GET /healthz`, Settings → Backup download, Settings → Instance (migration/version).

### SSO

1. Set OIDC env vars (issuer, client id/secret, redirect URL, role claim map).
2. Confirm **Settings → OIDC** shows issuer + sync flags.
3. Use **Claim preview** before enabling `OIDC_SYNC_ROLE` in production.
4. Keep a local admin account as break-glass if IdP is down (and a second admin for MFA break-glass).

### Sites & least privilege

1. Create **Locations**, assign subnets.
2. Scope users and API tokens to sites where possible (admin tokens stay global).
3. Non-admin users with **zero** sites see nothing (fail-closed) — assign memberships before go-live.
4. Prefer `readonly` / `reportview` tokens for dashboards and exporters.

### Retention & purge

| Setting | What it cleans |
|---------|----------------|
| Deleted IPs / subnets | Soft-deleted inventory after N days |
| Audit | Audit log rows |
| Webhook deliveries | Delivery history |

`0` disables automatic purge. Use **Purge now** only after confirming the count — hard purge is irreversible.

### Utilization alerts

1. **Settings → Utilization alerts**: global percent for `subnet.utilization_high` (0 disables).
2. On a subnet detail page, set an optional per-subnet override (blank = use global).
3. Optionally enable **SMTP alerts** so the same events email operators alongside webhooks.

### Integrations

1. Configure PowerDNS / Kea / SNMP under Settings; start with dry-run where available.
2. Run Kea **Sync now** after defining DHCP IP ranges; use SNMP **Enrich now** against a switch management address.
3. Check `/api/v1/integrations/deliveries` (or API clients) when write-back looks wrong.

### Backups

1. **Settings → Backup**: download JSON periodically (or after large imports).
2. **Preview restore** shows would-create / would-skip counts before you confirm merge.
3. Postgres volume/snapshots remain the primary disaster-recovery path; JSON backup is for config + inventory portability.
4. Treat backups that may contain integration metadata carefully; rotate SMTP/API secrets if a backup is exposed.

### Webhooks & reports

1. Create endpoints with the right **format** (`json` / `slack` / `teams`).
2. Subscribe to `reports.summary` for scheduled rollups; set interval under Settings.
3. Set **webhook mute hours** so noisy util/discovery events do not repeat for the same entity.
4. Watch **Delivery log**; retry failed posts after fixing the receiver.
5. Check header **Alerts** for in-app copies of recent events.

### Ops checks

- `GET /healthz` / `/api/v1/healthz` — status, version, database ping.
- Admin **Settings → Instance** — migration version, mute hours, login rate limit (8 fails / 15m per IP).

### Tokens & automation

- Create tokens with the minimum role + sites.
- Store the secret outside git; rotate by deleting and recreating.
- CSV / discovery / bulk APIs honor the same scope as the UI.

### Upgrades

1. Pull image/tag or rebuild.
2. Restart API — migrations run automatically (`db/migrations`).
3. Spot-check healthz, login, one subnet, discovery job, and a webhook test.
4. If UI is embedded, rebuild/restart the web or API image that serves `go:embed` assets.

---

## 19. Tips

- Soft-delete is recoverable until retention purge; hard purge is irreversible.
- Site-scoped search, IP history, discovery jobs, sweeps, CSV import/export, and reports never widen beyond your assigned sites; empty site membership for non-admins is fail-closed.
- Sweeps must target an inventory subnet you can access (arbitrary CIDR probes are rejected).
- Sparse IPv6 prefixes never get a full occupancy grid — register or claim hosts instead of sweeping the `/64`.
- Tag filters and report `location_id` URLs are safe to bookmark or share with colleagues who have access.
- Prefer API tokens with least privilege (readonly + sites) for automation.
- Search remembers recent queries locally and jumps straight to a single IP or subnet hit.
- Use the IP status grid on `/24`s for a quick occupancy glance before editing the table.
- Store MFA recovery codes offline; keep a second admin for break-glass.

---

## 20. Getting help

- API & ops: [README](../README.md)
- Security / hardening: [SECURITY.md](SECURITY.md)
- Health: `GET /api/v1/healthz` or `/healthz`
- Product issues: Wylde Technologies. Support offerings will be published on the company site.
