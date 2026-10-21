-- name: ListIPs :many
SELECT id, host(address)::text AS address, subnet_id, vrf_id, hostname, status, device_type_id, location_id, notes,
       reserved_until, last_seen_at, mac_address, open_ports, interface_name, switch_id, custom_fields, created_at, updated_at, deleted_at
FROM ip_addresses
WHERE deleted_at IS NULL
ORDER BY address;

-- name: ListIPsBySubnet :many
SELECT id, host(address)::text AS address, subnet_id, vrf_id, hostname, status, device_type_id, location_id, notes,
       reserved_until, last_seen_at, mac_address, open_ports, interface_name, switch_id, custom_fields, created_at, updated_at, deleted_at
FROM ip_addresses
WHERE subnet_id = $1 AND deleted_at IS NULL
ORDER BY address;

-- name: ListIPMapBySubnet :many
SELECT id, host(address)::text AS address, hostname, status, deleted_at
FROM ip_addresses
WHERE subnet_id = $1
ORDER BY address;

-- name: ListOccupiedAddressesBySubnet :many
-- Any live row (including available placeholders) blocks allocate-next.
SELECT host(address)::text AS address
FROM ip_addresses
WHERE subnet_id = $1
  AND deleted_at IS NULL;

-- name: ListUsedAddressesBySubnet :many
SELECT host(address)::text AS address
FROM ip_addresses
WHERE subnet_id = $1
  AND deleted_at IS NULL
  AND status IN ('assigned', 'reserved', 'dhcp')
  AND (reserved_until IS NULL OR reserved_until > now());

-- name: GetIP :one
SELECT id, host(address)::text AS address, subnet_id, vrf_id, hostname, status, device_type_id, location_id, notes,
       reserved_until, last_seen_at, mac_address, open_ports, interface_name, switch_id, custom_fields, created_at, updated_at, deleted_at
FROM ip_addresses
WHERE id = $1;

-- name: GetIPByAddress :one
SELECT id, host(address)::text AS address, subnet_id, vrf_id, hostname, status, device_type_id, location_id, notes,
       reserved_until, last_seen_at, mac_address, open_ports, interface_name, switch_id, custom_fields, created_at, updated_at, deleted_at
FROM ip_addresses
WHERE address = sqlc.arg(address)::inet
  AND vrf_id = sqlc.arg(vrf_id)
  AND deleted_at IS NULL;

-- name: GetIPByAddressAny :one
SELECT id, host(address)::text AS address, subnet_id, vrf_id, hostname, status, device_type_id, location_id, notes,
       reserved_until, last_seen_at, mac_address, open_ports, interface_name, switch_id, custom_fields, created_at, updated_at, deleted_at
FROM ip_addresses
WHERE address = sqlc.arg(address)::inet
  AND vrf_id = sqlc.arg(vrf_id)
ORDER BY deleted_at NULLS FIRST
LIMIT 1;

-- name: CreateIP :one
INSERT INTO ip_addresses (
    address, subnet_id, vrf_id, hostname, status, device_type_id, location_id, notes,
    reserved_until, last_seen_at, mac_address, open_ports, interface_name, switch_id, custom_fields
) VALUES (
    sqlc.arg(address)::inet,
    sqlc.arg(subnet_id),
    sqlc.arg(vrf_id),
    sqlc.arg(hostname),
    sqlc.arg(status),
    sqlc.arg(device_type_id),
    sqlc.arg(location_id),
    sqlc.arg(notes),
    sqlc.arg(reserved_until),
    sqlc.arg(last_seen_at),
    sqlc.arg(mac_address),
    sqlc.arg(open_ports),
    sqlc.arg(interface_name),
    sqlc.arg(switch_id),
    sqlc.arg(custom_fields)
)
RETURNING id, host(address)::text AS address, subnet_id, vrf_id, hostname, status, device_type_id, location_id, notes,
          reserved_until, last_seen_at, mac_address, open_ports, interface_name, switch_id, custom_fields, created_at, updated_at, deleted_at;

-- name: UpdateIP :one
UPDATE ip_addresses
SET address = sqlc.arg(address)::inet,
    subnet_id = sqlc.arg(subnet_id),
    vrf_id = sqlc.arg(vrf_id),
    hostname = sqlc.arg(hostname),
    status = sqlc.arg(status),
    device_type_id = sqlc.arg(device_type_id),
    location_id = sqlc.arg(location_id),
    notes = sqlc.arg(notes),
    reserved_until = sqlc.arg(reserved_until),
    last_seen_at = sqlc.arg(last_seen_at),
    mac_address = sqlc.arg(mac_address),
    open_ports = sqlc.arg(open_ports),
    interface_name = sqlc.arg(interface_name),
    switch_id = sqlc.arg(switch_id),
    custom_fields = sqlc.arg(custom_fields),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING id, host(address)::text AS address, subnet_id, vrf_id, hostname, status, device_type_id, location_id, notes,
          reserved_until, last_seen_at, mac_address, open_ports, interface_name, switch_id, custom_fields, created_at, updated_at, deleted_at;

-- name: SoftDeleteIP :one
UPDATE ip_addresses
SET deleted_at = now(),
    updated_at = now()
WHERE id = $1
  AND deleted_at IS NULL
RETURNING id, host(address)::text AS address, subnet_id, vrf_id, hostname, status, device_type_id, location_id, notes,
          reserved_until, last_seen_at, mac_address, open_ports, interface_name, switch_id, custom_fields, created_at, updated_at, deleted_at;

-- name: RestoreIP :one
UPDATE ip_addresses
SET deleted_at = NULL,
    subnet_id = sqlc.arg(subnet_id),
    vrf_id = sqlc.arg(vrf_id),
    hostname = sqlc.arg(hostname),
    status = sqlc.arg(status),
    device_type_id = sqlc.arg(device_type_id),
    location_id = sqlc.arg(location_id),
    notes = sqlc.arg(notes),
    reserved_until = sqlc.arg(reserved_until),
    last_seen_at = sqlc.arg(last_seen_at),
    mac_address = sqlc.arg(mac_address),
    open_ports = sqlc.arg(open_ports),
    interface_name = sqlc.arg(interface_name),
    switch_id = sqlc.arg(switch_id),
    custom_fields = sqlc.arg(custom_fields),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, host(address)::text AS address, subnet_id, vrf_id, hostname, status, device_type_id, location_id, notes,
          reserved_until, last_seen_at, mac_address, open_ports, interface_name, switch_id, custom_fields, created_at, updated_at, deleted_at;

-- name: DeleteIP :exec
DELETE FROM ip_addresses WHERE id = $1;

-- name: SearchIPs :many
SELECT id, host(address)::text AS address, subnet_id, vrf_id, hostname, status, device_type_id, location_id, notes,
       reserved_until, last_seen_at, mac_address, open_ports, interface_name, switch_id, custom_fields, created_at, updated_at, deleted_at
FROM ip_addresses
WHERE deleted_at IS NULL
  AND (
    host(address)::text ILIKE '%' || sqlc.arg(query)::text || '%'
    OR hostname ILIKE '%' || sqlc.arg(query)::text || '%'
    OR notes ILIKE '%' || sqlc.arg(query)::text || '%'
    OR mac_address ILIKE '%' || sqlc.arg(query)::text || '%'
  )
  AND (
    NOT sqlc.arg(scoped)::bool
    OR location_id = ANY(sqlc.arg(location_ids)::uuid[])
  )
ORDER BY address
LIMIT 50;

-- name: ListDeletedIPs :many
SELECT id, host(address)::text AS address, subnet_id, vrf_id, hostname, status, device_type_id, location_id, notes,
       reserved_until, last_seen_at, mac_address, open_ports, interface_name, switch_id, custom_fields, created_at, updated_at, deleted_at
FROM ip_addresses
WHERE deleted_at IS NOT NULL
  AND (
    sqlc.arg(query)::text = ''
    OR host(address)::text ILIKE '%' || sqlc.arg(query) || '%'
    OR hostname ILIKE '%' || sqlc.arg(query) || '%'
  )
ORDER BY deleted_at DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountDeletedIPs :one
SELECT count(*) FROM ip_addresses
WHERE deleted_at IS NOT NULL
  AND (
    sqlc.arg(query)::text = ''
    OR host(address)::text ILIKE '%' || sqlc.arg(query) || '%'
    OR hostname ILIKE '%' || sqlc.arg(query) || '%'
  );

-- name: PurgeDeletedIPsBefore :execrows
DELETE FROM ip_addresses
WHERE deleted_at IS NOT NULL
  AND deleted_at < sqlc.arg(before)::timestamptz;

-- name: ListIPsByIDs :many
SELECT id, host(address)::text AS address, subnet_id, vrf_id, hostname, status, device_type_id, location_id, notes,
       reserved_until, last_seen_at, mac_address, open_ports, interface_name, switch_id, custom_fields, created_at, updated_at, deleted_at
FROM ip_addresses
WHERE deleted_at IS NULL
  AND id = ANY(sqlc.arg(ids)::uuid[])
ORDER BY address;

-- name: BulkUpdateIPStatus :exec
UPDATE ip_addresses
SET status = sqlc.arg(status),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;

-- name: BulkUpdateIPDeviceType :exec
UPDATE ip_addresses
SET device_type_id = sqlc.arg(device_type_id),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;

-- name: BulkSetIPCustomFields :exec
UPDATE ip_addresses
SET custom_fields = sqlc.arg(custom_fields),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;
