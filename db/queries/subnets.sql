-- name: ListSubnets :many
SELECT id, cidr::text AS cidr, name, description, location_id, vrf_id, vlan_id, parent_id, tags, custom_fields, created_at, updated_at
FROM subnets
WHERE deleted_at IS NULL
ORDER BY cidr;

-- name: GetSubnet :one
SELECT id, cidr::text AS cidr, name, description, location_id, vrf_id, vlan_id, parent_id, tags, custom_fields, created_at, updated_at, deleted_at
FROM subnets
WHERE id = $1;

-- name: GetSubnetByCIDR :one
SELECT id, cidr::text AS cidr, name, description, location_id, vrf_id, vlan_id, parent_id, tags, custom_fields, created_at, updated_at, deleted_at
FROM subnets
WHERE deleted_at IS NULL
  AND cidr = sqlc.arg(cidr)::cidr
  AND vrf_id = sqlc.arg(vrf_id);

-- name: CreateSubnet :one
INSERT INTO subnets (cidr, name, description, location_id, vrf_id, vlan_id, parent_id, tags, custom_fields)
VALUES (
    sqlc.arg(cidr)::cidr,
    sqlc.arg(name),
    sqlc.arg(description),
    sqlc.arg(location_id),
    sqlc.arg(vrf_id),
    sqlc.arg(vlan_id),
    sqlc.arg(parent_id),
    sqlc.arg(tags),
    sqlc.arg(custom_fields)
)
RETURNING id, cidr::text AS cidr, name, description, location_id, vrf_id, vlan_id, parent_id, tags, custom_fields, created_at, updated_at;

-- name: UpdateSubnet :one
UPDATE subnets
SET cidr = sqlc.arg(cidr)::cidr,
    name = sqlc.arg(name),
    description = sqlc.arg(description),
    location_id = sqlc.arg(location_id),
    vrf_id = sqlc.arg(vrf_id),
    vlan_id = sqlc.arg(vlan_id),
    parent_id = sqlc.arg(parent_id),
    tags = sqlc.arg(tags),
    custom_fields = sqlc.arg(custom_fields),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING id, cidr::text AS cidr, name, description, location_id, vrf_id, vlan_id, parent_id, tags, custom_fields, created_at, updated_at;

-- name: SoftDeleteSubnet :one
UPDATE subnets
SET deleted_at = now(),
    updated_at = now()
WHERE id = $1
  AND deleted_at IS NULL
RETURNING id, cidr::text AS cidr, name, description, location_id, vrf_id, vlan_id, parent_id, tags, custom_fields, created_at, updated_at, deleted_at;

-- name: RestoreSubnet :one
UPDATE subnets
SET deleted_at = NULL,
    updated_at = now()
WHERE id = $1
  AND deleted_at IS NOT NULL
RETURNING id, cidr::text AS cidr, name, description, location_id, vrf_id, vlan_id, parent_id, tags, custom_fields, created_at, updated_at, deleted_at;

-- name: DeleteSubnet :exec
DELETE FROM subnets WHERE id = $1;

-- name: ListDeletedSubnets :many
SELECT id, cidr::text AS cidr, name, description, location_id, vrf_id, vlan_id, parent_id, tags, custom_fields, created_at, updated_at, deleted_at
FROM subnets
WHERE deleted_at IS NOT NULL
  AND (
    sqlc.arg(query)::text = ''
    OR cidr::text ILIKE '%' || sqlc.arg(query) || '%'
    OR name ILIKE '%' || sqlc.arg(query) || '%'
  )
ORDER BY deleted_at DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountDeletedSubnets :one
SELECT count(*) FROM subnets
WHERE deleted_at IS NOT NULL
  AND (
    sqlc.arg(query)::text = ''
    OR cidr::text ILIKE '%' || sqlc.arg(query) || '%'
    OR name ILIKE '%' || sqlc.arg(query) || '%'
  );

-- name: PurgeDeletedSubnetsBefore :execrows
DELETE FROM subnets
WHERE deleted_at IS NOT NULL
  AND deleted_at < sqlc.arg(before)::timestamptz;

-- name: ListChildSubnets :many
SELECT id, cidr::text AS cidr, name, description, location_id, vrf_id, vlan_id, parent_id, tags, custom_fields, created_at, updated_at
FROM subnets
WHERE parent_id = $1
  AND deleted_at IS NULL
ORDER BY cidr;

-- name: ListSubnetsByLocation :many
SELECT id, cidr::text AS cidr, name, description, location_id, vrf_id, vlan_id, parent_id, tags, custom_fields, created_at, updated_at
FROM subnets
WHERE location_id = $1
  AND deleted_at IS NULL
ORDER BY cidr;

-- name: FindOverlappingSubnets :many
SELECT id, cidr::text AS cidr, name, description, location_id, vrf_id, vlan_id, parent_id, tags, custom_fields, created_at, updated_at
FROM subnets
WHERE deleted_at IS NULL
  AND vrf_id = sqlc.arg(vrf_id)
  AND cidr && sqlc.arg(cidr)::cidr
  AND (sqlc.narg(exclude_id)::uuid IS NULL OR id <> sqlc.narg(exclude_id));

-- name: FindTightestParent :one
SELECT id, cidr::text AS cidr, name, description, location_id, vrf_id, vlan_id, parent_id, tags, custom_fields, created_at, updated_at
FROM subnets
WHERE deleted_at IS NULL
  AND vrf_id = sqlc.arg(vrf_id)
  AND cidr >> sqlc.arg(cidr)::cidr
ORDER BY masklen(cidr) DESC
LIMIT 1;

-- name: ListSubnetIPCounts :many
SELECT s.id AS subnet_id, count(i.id)::bigint AS used_count
FROM subnets s
LEFT JOIN ip_addresses i ON i.subnet_id = s.id
  AND i.deleted_at IS NULL
  AND i.status IN ('assigned', 'reserved', 'dhcp')
  AND (i.reserved_until IS NULL OR i.reserved_until > now())
WHERE s.deleted_at IS NULL
GROUP BY s.id;

-- name: SearchSubnets :many
SELECT id, cidr::text AS cidr, name, description, location_id, vrf_id, vlan_id, parent_id, tags, custom_fields, created_at, updated_at
FROM subnets
WHERE deleted_at IS NULL
  AND (
    cidr::text ILIKE '%' || sqlc.arg(query)::text || '%'
    OR name ILIKE '%' || sqlc.arg(query)::text || '%'
    OR description ILIKE '%' || sqlc.arg(query)::text || '%'
    OR array_to_string(tags, ',') ILIKE '%' || sqlc.arg(query)::text || '%'
  )
  AND (
    NOT sqlc.arg(scoped)::bool
    OR location_id = ANY(sqlc.arg(location_ids)::uuid[])
  )
ORDER BY cidr
LIMIT 50;

-- name: SetSubnetParent :exec
UPDATE subnets
SET parent_id = sqlc.arg(parent_id),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;

-- name: FindSubnetContainingIP :one
SELECT id, cidr::text AS cidr, name, description, location_id, vrf_id, vlan_id, parent_id, tags, custom_fields, created_at, updated_at
FROM subnets
WHERE deleted_at IS NULL
  AND vrf_id = sqlc.arg(vrf_id)
  AND sqlc.arg(address)::inet <<= cidr
ORDER BY masklen(cidr) DESC
LIMIT 1;

-- name: ListSubnetsByIDs :many
SELECT id, cidr::text AS cidr, name, description, location_id, vrf_id, vlan_id, parent_id, tags, custom_fields, created_at, updated_at
FROM subnets
WHERE deleted_at IS NULL
  AND id = ANY(sqlc.arg(ids)::uuid[])
ORDER BY cidr;

-- name: BulkSetSubnetTags :exec
UPDATE subnets
SET tags = sqlc.arg(tags),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;

-- name: BulkSetSubnetLocation :exec
UPDATE subnets
SET location_id = sqlc.arg(location_id),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;

-- name: BulkSetSubnetCustomFields :exec
UPDATE subnets
SET custom_fields = sqlc.arg(custom_fields),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;
