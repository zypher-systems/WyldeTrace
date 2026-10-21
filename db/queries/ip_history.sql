-- name: CreateIPHistory :one
INSERT INTO ip_history (
    ip_id, address, subnet_id, hostname, status, device_type_id, location_id,
    notes, mac_address, open_ports, reserved_until, last_seen_at,
    action, actor_email, details
) VALUES (
    sqlc.arg(ip_id),
    sqlc.arg(address)::inet,
    sqlc.arg(subnet_id),
    sqlc.arg(hostname),
    sqlc.arg(status),
    sqlc.arg(device_type_id),
    sqlc.arg(location_id),
    sqlc.arg(notes),
    sqlc.arg(mac_address),
    sqlc.arg(open_ports),
    sqlc.arg(reserved_until),
    sqlc.arg(last_seen_at),
    sqlc.arg(action),
    sqlc.arg(actor_email),
    sqlc.arg(details)
)
RETURNING id, ip_id, host(address)::text AS address, subnet_id, hostname, status, device_type_id, location_id,
    notes, mac_address, open_ports, reserved_until, last_seen_at, action, actor_email, details, created_at;

-- name: ListIPHistoryByAddress :many
SELECT id, ip_id, host(address)::text AS address, subnet_id, hostname, status, device_type_id, location_id,
    notes, mac_address, open_ports, reserved_until, last_seen_at, action, actor_email, details, created_at
FROM ip_history
WHERE host(address)::text = sqlc.arg(address)
  AND (
    NOT sqlc.arg(scoped)::bool
    OR location_id = ANY(sqlc.arg(location_ids)::uuid[])
  )
ORDER BY created_at DESC
LIMIT sqlc.arg(row_limit);

-- name: ListIPHistoryByIPID :many
SELECT id, ip_id, host(address)::text AS address, subnet_id, hostname, status, device_type_id, location_id,
    notes, mac_address, open_ports, reserved_until, last_seen_at, action, actor_email, details, created_at
FROM ip_history
WHERE ip_id = sqlc.arg(ip_id)
  AND (
    NOT sqlc.arg(scoped)::bool
    OR location_id = ANY(sqlc.arg(location_ids)::uuid[])
  )
ORDER BY created_at DESC
LIMIT sqlc.arg(row_limit);

-- name: ListIPHistoryFiltered :many
SELECT id, ip_id, host(address)::text AS address, subnet_id, hostname, status, device_type_id, location_id,
    notes, mac_address, open_ports, reserved_until, last_seen_at, action, actor_email, details, created_at
FROM ip_history
WHERE (sqlc.arg(address)::text = '' OR host(address)::text = sqlc.arg(address))
  AND (sqlc.arg(action)::text = '' OR action ILIKE '%' || sqlc.arg(action) || '%')
  AND (sqlc.arg(actor)::text = '' OR actor_email ILIKE '%' || sqlc.arg(actor) || '%')
  AND (sqlc.narg(from_ts)::timestamptz IS NULL OR created_at >= sqlc.narg(from_ts))
  AND (sqlc.narg(to_ts)::timestamptz IS NULL OR created_at <= sqlc.narg(to_ts))
  AND (
    NOT sqlc.arg(scoped)::bool
    OR location_id = ANY(sqlc.arg(location_ids)::uuid[])
  )
ORDER BY
  CASE WHEN sqlc.arg(sort_by)::text = 'address' AND sqlc.arg(sort_dir)::text = 'asc' THEN address END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'address' AND sqlc.arg(sort_dir)::text = 'desc' THEN address END DESC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'action' AND sqlc.arg(sort_dir)::text = 'asc' THEN action END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'action' AND sqlc.arg(sort_dir)::text = 'desc' THEN action END DESC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'actor_email' AND sqlc.arg(sort_dir)::text = 'asc' THEN actor_email END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'actor_email' AND sqlc.arg(sort_dir)::text = 'desc' THEN actor_email END DESC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'created_at' AND sqlc.arg(sort_dir)::text = 'asc' THEN created_at END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'created_at' AND sqlc.arg(sort_dir)::text = 'desc' THEN created_at END DESC NULLS LAST,
  created_at DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountIPHistoryFiltered :one
SELECT count(*) FROM ip_history
WHERE (sqlc.arg(address)::text = '' OR host(address)::text = sqlc.arg(address))
  AND (sqlc.arg(action)::text = '' OR action ILIKE '%' || sqlc.arg(action) || '%')
  AND (sqlc.arg(actor)::text = '' OR actor_email ILIKE '%' || sqlc.arg(actor) || '%')
  AND (sqlc.narg(from_ts)::timestamptz IS NULL OR created_at >= sqlc.narg(from_ts))
  AND (sqlc.narg(to_ts)::timestamptz IS NULL OR created_at <= sqlc.narg(to_ts))
  AND (
    NOT sqlc.arg(scoped)::bool
    OR location_id = ANY(sqlc.arg(location_ids)::uuid[])
  );
