-- name: CountSubnetsFiltered :one
SELECT count(*) FROM subnets
WHERE deleted_at IS NULL
AND (
    sqlc.arg(query)::text = ''
    OR cidr::text ILIKE '%' || sqlc.arg(query) || '%'
    OR name ILIKE '%' || sqlc.arg(query) || '%'
    OR description ILIKE '%' || sqlc.arg(query) || '%'
    OR array_to_string(tags, ',') ILIKE '%' || sqlc.arg(query) || '%'
)
AND (
    cardinality(sqlc.arg(tags)::text[]) = 0
    OR (
        CASE
            WHEN sqlc.arg(tag_match)::text = 'or' THEN tags && sqlc.arg(tags)::text[]
            ELSE tags @> sqlc.arg(tags)::text[]
        END
    )
)
AND (
    NOT sqlc.arg(scoped)::bool
    OR location_id = ANY(sqlc.arg(location_ids)::uuid[])
);

-- name: ListSubnetsPaged :many
SELECT id, cidr::text AS cidr, name, description, location_id, vrf_id, vlan_id, parent_id, tags, custom_fields, created_at, updated_at
FROM subnets
WHERE deleted_at IS NULL
AND (
    sqlc.arg(query)::text = ''
    OR cidr::text ILIKE '%' || sqlc.arg(query) || '%'
    OR name ILIKE '%' || sqlc.arg(query) || '%'
    OR description ILIKE '%' || sqlc.arg(query) || '%'
    OR array_to_string(tags, ',') ILIKE '%' || sqlc.arg(query) || '%'
)
AND (
    cardinality(sqlc.arg(tags)::text[]) = 0
    OR (
        CASE
            WHEN sqlc.arg(tag_match)::text = 'or' THEN tags && sqlc.arg(tags)::text[]
            ELSE tags @> sqlc.arg(tags)::text[]
        END
    )
)
AND (
    NOT sqlc.arg(scoped)::bool
    OR location_id = ANY(sqlc.arg(location_ids)::uuid[])
)
ORDER BY
  CASE WHEN sqlc.arg(sort_by)::text = 'name' AND sqlc.arg(sort_dir)::text = 'asc' THEN name END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'name' AND sqlc.arg(sort_dir)::text = 'desc' THEN name END DESC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'cidr' AND sqlc.arg(sort_dir)::text = 'asc' THEN cidr END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'cidr' AND sqlc.arg(sort_dir)::text = 'desc' THEN cidr END DESC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'created_at' AND sqlc.arg(sort_dir)::text = 'asc' THEN created_at END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'created_at' AND sqlc.arg(sort_dir)::text = 'desc' THEN created_at END DESC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'updated_at' AND sqlc.arg(sort_dir)::text = 'asc' THEN updated_at END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'updated_at' AND sqlc.arg(sort_dir)::text = 'desc' THEN updated_at END DESC NULLS LAST,
  cidr ASC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: ListSubnetTags :many
SELECT DISTINCT lower(t) AS tag
FROM subnets, unnest(tags) AS t
WHERE deleted_at IS NULL
AND (
    NOT sqlc.arg(scoped)::bool
    OR location_id = ANY(sqlc.arg(location_ids)::uuid[])
)
AND t <> ''
ORDER BY tag;

-- name: CountIPsFiltered :one
SELECT count(*) FROM ip_addresses
WHERE deleted_at IS NULL
AND (
    sqlc.narg(subnet_id)::uuid IS NULL OR subnet_id = sqlc.narg(subnet_id)
)
AND (
    sqlc.arg(status)::text = '' OR status::text = sqlc.arg(status)
)
AND (
    sqlc.arg(query)::text = ''
    OR host(address)::text ILIKE '%' || sqlc.arg(query) || '%'
    OR hostname ILIKE '%' || sqlc.arg(query) || '%'
    OR notes ILIKE '%' || sqlc.arg(query) || '%'
    OR mac_address ILIKE '%' || sqlc.arg(query) || '%'
)
AND (
    NOT sqlc.arg(scoped)::bool
    OR location_id = ANY(sqlc.arg(location_ids)::uuid[])
);

-- name: ListIPsPaged :many
SELECT id, host(address)::text AS address, subnet_id, vrf_id, hostname, status, device_type_id, location_id, notes,
       reserved_until, last_seen_at, mac_address, open_ports, interface_name, switch_id, custom_fields, created_at, updated_at, deleted_at
FROM ip_addresses
WHERE deleted_at IS NULL
AND (
    sqlc.narg(subnet_id)::uuid IS NULL OR subnet_id = sqlc.narg(subnet_id)
)
AND (
    sqlc.arg(status)::text = '' OR status::text = sqlc.arg(status)
)
AND (
    sqlc.arg(query)::text = ''
    OR host(address)::text ILIKE '%' || sqlc.arg(query) || '%'
    OR hostname ILIKE '%' || sqlc.arg(query) || '%'
    OR notes ILIKE '%' || sqlc.arg(query) || '%'
    OR mac_address ILIKE '%' || sqlc.arg(query) || '%'
)
AND (
    NOT sqlc.arg(scoped)::bool
    OR location_id = ANY(sqlc.arg(location_ids)::uuid[])
)
ORDER BY
  CASE WHEN sqlc.arg(sort_by)::text = 'hostname' AND sqlc.arg(sort_dir)::text = 'asc' THEN hostname END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'hostname' AND sqlc.arg(sort_dir)::text = 'desc' THEN hostname END DESC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'status' AND sqlc.arg(sort_dir)::text = 'asc' THEN status::text END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'status' AND sqlc.arg(sort_dir)::text = 'desc' THEN status::text END DESC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'address' AND sqlc.arg(sort_dir)::text = 'asc' THEN address END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'address' AND sqlc.arg(sort_dir)::text = 'desc' THEN address END DESC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'created_at' AND sqlc.arg(sort_dir)::text = 'asc' THEN created_at END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'created_at' AND sqlc.arg(sort_dir)::text = 'desc' THEN created_at END DESC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'updated_at' AND sqlc.arg(sort_dir)::text = 'asc' THEN updated_at END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'updated_at' AND sqlc.arg(sort_dir)::text = 'desc' THEN updated_at END DESC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'last_seen_at' AND sqlc.arg(sort_dir)::text = 'asc' THEN last_seen_at END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'last_seen_at' AND sqlc.arg(sort_dir)::text = 'desc' THEN last_seen_at END DESC NULLS LAST,
  address ASC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountAuditLogs :one
SELECT count(*) FROM audit_logs
WHERE (
    sqlc.arg(action)::text = '' OR action ILIKE '%' || sqlc.arg(action) || '%'
)
AND (
    sqlc.arg(actor)::text = '' OR actor_email ILIKE '%' || sqlc.arg(actor) || '%'
)
AND (
    sqlc.narg(from_ts)::timestamptz IS NULL OR created_at >= sqlc.narg(from_ts)
)
AND (
    sqlc.narg(to_ts)::timestamptz IS NULL OR created_at <= sqlc.narg(to_ts)
);

-- name: ListAuditLogsPaged :many
SELECT * FROM audit_logs
WHERE (
    sqlc.arg(action)::text = '' OR action ILIKE '%' || sqlc.arg(action) || '%'
)
AND (
    sqlc.arg(actor)::text = '' OR actor_email ILIKE '%' || sqlc.arg(actor) || '%'
)
AND (
    sqlc.narg(from_ts)::timestamptz IS NULL OR created_at >= sqlc.narg(from_ts)
)
AND (
    sqlc.narg(to_ts)::timestamptz IS NULL OR created_at <= sqlc.narg(to_ts)
)
ORDER BY
  CASE WHEN sqlc.arg(sort_by)::text = 'action' AND sqlc.arg(sort_dir)::text = 'asc' THEN action END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'action' AND sqlc.arg(sort_dir)::text = 'desc' THEN action END DESC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'actor_email' AND sqlc.arg(sort_dir)::text = 'asc' THEN actor_email END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'actor_email' AND sqlc.arg(sort_dir)::text = 'desc' THEN actor_email END DESC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'entity_type' AND sqlc.arg(sort_dir)::text = 'asc' THEN entity_type END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'entity_type' AND sqlc.arg(sort_dir)::text = 'desc' THEN entity_type END DESC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'created_at' AND sqlc.arg(sort_dir)::text = 'asc' THEN created_at END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'created_at' AND sqlc.arg(sort_dir)::text = 'desc' THEN created_at END DESC NULLS LAST,
  created_at DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);
