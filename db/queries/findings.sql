-- name: GetOpenFinding :one
SELECT id, subnet_id, host(address)::text AS address, kind, hostname, mac_address, open_ports, interface_name, switch_id,
       db_ip_id, db_status, job_id, first_seen_at, last_seen_at, resolved_at, resolve_action
FROM discovery_findings
WHERE subnet_id = sqlc.arg(subnet_id)
  AND address = sqlc.arg(address)::inet
  AND kind = sqlc.arg(kind)
  AND resolved_at IS NULL
LIMIT 1;

-- name: InsertFinding :one
INSERT INTO discovery_findings (
    subnet_id, address, kind, hostname, mac_address, open_ports, interface_name, switch_id, db_ip_id, db_status, job_id
) VALUES (
    sqlc.arg(subnet_id),
    sqlc.arg(address)::inet,
    sqlc.arg(kind),
    sqlc.arg(hostname),
    sqlc.arg(mac_address),
    sqlc.arg(open_ports),
    sqlc.arg(interface_name),
    sqlc.arg(switch_id),
    sqlc.arg(db_ip_id),
    sqlc.arg(db_status),
    sqlc.arg(job_id)
)
RETURNING id, subnet_id, host(address)::text AS address, kind, hostname, mac_address, open_ports, interface_name, switch_id,
          db_ip_id, db_status, job_id, first_seen_at, last_seen_at, resolved_at, resolve_action;

-- name: TouchFinding :one
UPDATE discovery_findings
SET hostname = sqlc.arg(hostname),
    mac_address = sqlc.arg(mac_address),
    open_ports = sqlc.arg(open_ports),
    interface_name = sqlc.arg(interface_name),
    switch_id = sqlc.arg(switch_id),
    db_ip_id = sqlc.arg(db_ip_id),
    db_status = sqlc.arg(db_status),
    job_id = COALESCE(sqlc.arg(job_id), job_id),
    last_seen_at = now()
WHERE id = sqlc.arg(id) AND resolved_at IS NULL
RETURNING id, subnet_id, host(address)::text AS address, kind, hostname, mac_address, open_ports, interface_name, switch_id,
          db_ip_id, db_status, job_id, first_seen_at, last_seen_at, resolved_at, resolve_action;

-- name: ListOpenFindings :many
SELECT f.id, f.subnet_id, host(f.address)::text AS address, f.kind, f.hostname, f.mac_address, f.open_ports, f.interface_name, f.switch_id,
       f.db_ip_id, f.db_status, f.job_id, f.first_seen_at, f.last_seen_at, f.resolved_at, f.resolve_action
FROM discovery_findings f
JOIN subnets s ON s.id = f.subnet_id AND s.deleted_at IS NULL
WHERE f.resolved_at IS NULL
  AND (sqlc.arg(kind)::text = '' OR f.kind = sqlc.arg(kind))
  AND (sqlc.narg(subnet_id)::uuid IS NULL OR f.subnet_id = sqlc.narg(subnet_id))
  AND (sqlc.narg(location_id)::uuid IS NULL OR s.location_id = sqlc.narg(location_id))
  AND (
    NOT sqlc.arg(scoped)::bool
    OR s.location_id = ANY(sqlc.arg(location_ids)::uuid[])
  )
ORDER BY
  CASE WHEN sqlc.arg(sort_by)::text = 'address' AND sqlc.arg(sort_dir)::text = 'asc' THEN f.address END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'address' AND sqlc.arg(sort_dir)::text = 'desc' THEN f.address END DESC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'kind' AND sqlc.arg(sort_dir)::text = 'asc' THEN f.kind END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'kind' AND sqlc.arg(sort_dir)::text = 'desc' THEN f.kind END DESC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'first_seen_at' AND sqlc.arg(sort_dir)::text = 'asc' THEN f.first_seen_at END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'first_seen_at' AND sqlc.arg(sort_dir)::text = 'desc' THEN f.first_seen_at END DESC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'last_seen_at' AND sqlc.arg(sort_dir)::text = 'asc' THEN f.last_seen_at END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'last_seen_at' AND sqlc.arg(sort_dir)::text = 'desc' THEN f.last_seen_at END DESC NULLS LAST,
  f.last_seen_at DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountOpenFindings :one
SELECT count(*) FROM discovery_findings f
JOIN subnets s ON s.id = f.subnet_id AND s.deleted_at IS NULL
WHERE f.resolved_at IS NULL
  AND (sqlc.arg(kind)::text = '' OR f.kind = sqlc.arg(kind))
  AND (sqlc.narg(subnet_id)::uuid IS NULL OR f.subnet_id = sqlc.narg(subnet_id))
  AND (sqlc.narg(location_id)::uuid IS NULL OR s.location_id = sqlc.narg(location_id))
  AND (
    NOT sqlc.arg(scoped)::bool
    OR s.location_id = ANY(sqlc.arg(location_ids)::uuid[])
  );

-- name: CountOpenFindingsByKind :many
SELECT kind, count(*)::bigint AS count
FROM discovery_findings
WHERE resolved_at IS NULL
GROUP BY kind
ORDER BY kind;

-- name: LatestOpenFindingSeenAt :one
SELECT last_seen_at
FROM discovery_findings
WHERE resolved_at IS NULL
ORDER BY last_seen_at DESC
LIMIT 1;

-- name: GetFinding :one
SELECT id, subnet_id, host(address)::text AS address, kind, hostname, mac_address, open_ports, interface_name, switch_id,
       db_ip_id, db_status, job_id, first_seen_at, last_seen_at, resolved_at, resolve_action
FROM discovery_findings
WHERE id = $1;

-- name: ResolveFinding :one
UPDATE discovery_findings
SET resolved_at = now(),
    resolve_action = sqlc.arg(resolve_action)
WHERE id = sqlc.arg(id) AND resolved_at IS NULL
RETURNING id, subnet_id, host(address)::text AS address, kind, hostname, mac_address, open_ports, interface_name, switch_id,
          db_ip_id, db_status, job_id, first_seen_at, last_seen_at, resolved_at, resolve_action;

-- name: ResolveFindingsByAddress :exec
UPDATE discovery_findings
SET resolved_at = now(),
    resolve_action = sqlc.arg(resolve_action)
WHERE subnet_id = sqlc.arg(subnet_id)
  AND address = sqlc.arg(address)::inet
  AND resolved_at IS NULL;
