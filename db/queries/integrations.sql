-- name: CreateIntegrationDelivery :one
INSERT INTO integration_deliveries (provider, action, status, status_code, error, dry_run, request_body, response_body)
VALUES (
    sqlc.arg(provider),
    sqlc.arg(action),
    sqlc.arg(status),
    sqlc.arg(status_code),
    sqlc.arg(error),
    sqlc.arg(dry_run),
    sqlc.arg(request_body),
    sqlc.arg(response_body)
)
RETURNING *;

-- name: ListIntegrationDeliveries :many
SELECT * FROM integration_deliveries
WHERE (sqlc.arg(provider)::text = '' OR provider = sqlc.arg(provider))
ORDER BY created_at DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountIntegrationDeliveries :one
SELECT count(*) FROM integration_deliveries
WHERE (sqlc.arg(provider)::text = '' OR provider = sqlc.arg(provider));

-- name: PurgeIntegrationDeliveriesBefore :execrows
DELETE FROM integration_deliveries
WHERE created_at < sqlc.arg(before)::timestamptz;

-- name: ListIPRangesByRole :many
SELECT id, subnet_id, name, host(start_address)::text AS start_address, host(end_address)::text AS end_address,
       role, description, created_at, updated_at
FROM ip_ranges
WHERE role = sqlc.arg(role)
ORDER BY start_address;

-- name: ListAllIPRanges :many
SELECT id, subnet_id, name, host(start_address)::text AS start_address, host(end_address)::text AS end_address,
       role, description, created_at, updated_at
FROM ip_ranges
ORDER BY start_address;

-- name: ListReservedOrAssignedIPs :many
SELECT id, host(address)::text AS address, subnet_id, vrf_id, hostname, status, mac_address, interface_name, switch_id, location_id
FROM ip_addresses
WHERE deleted_at IS NULL
  AND status IN ('reserved', 'assigned', 'dhcp')
ORDER BY address;

-- name: MoveIPToSubnet :exec
UPDATE ip_addresses AS i
SET subnet_id = sqlc.arg(subnet_id),
    vrf_id = (SELECT s.vrf_id FROM subnets s WHERE s.id = sqlc.arg(subnet_id)),
    updated_at = now()
WHERE i.id = sqlc.arg(id)
  AND i.deleted_at IS NULL;

-- name: UpdateIPEnrichment :exec
UPDATE ip_addresses
SET interface_name = sqlc.arg(interface_name),
    switch_id = sqlc.arg(switch_id),
    mac_address = CASE WHEN sqlc.arg(mac_address) = '' THEN mac_address ELSE sqlc.arg(mac_address) END,
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;

-- name: UpdateFindingEnrichment :exec
UPDATE discovery_findings
SET interface_name = sqlc.arg(interface_name),
    switch_id = sqlc.arg(switch_id),
    mac_address = CASE WHEN sqlc.arg(mac_address) = '' THEN mac_address ELSE sqlc.arg(mac_address) END
WHERE id = sqlc.arg(id)
  AND resolved_at IS NULL;
