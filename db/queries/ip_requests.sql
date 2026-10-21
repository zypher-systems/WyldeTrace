-- name: CreateIPRequest :one
INSERT INTO ip_requests (
    requester_id, subnet_id, range_id, hostname, notes, requested_address
) VALUES (
    sqlc.arg(requester_id),
    sqlc.arg(subnet_id),
    sqlc.narg(range_id),
    sqlc.arg(hostname),
    sqlc.arg(notes),
    sqlc.narg(requested_address)::inet
)
RETURNING id, requester_id, subnet_id, range_id, hostname, notes,
          CASE WHEN requested_address IS NULL THEN '' ELSE host(requested_address)::text END AS requested_address,
          status, allocated_ip_id, reviewed_by, review_notes,
          created_at, updated_at, reviewed_at;

-- name: GetIPRequest :one
SELECT id, requester_id, subnet_id, range_id, hostname, notes,
       CASE WHEN requested_address IS NULL THEN '' ELSE host(requested_address)::text END AS requested_address,
       status, allocated_ip_id, reviewed_by, review_notes,
       created_at, updated_at, reviewed_at
FROM ip_requests
WHERE id = $1;

-- name: CountIPRequests :one
SELECT count(*) FROM ip_requests
WHERE (
    sqlc.arg(status)::text = '' OR status = sqlc.arg(status)
)
AND (
    sqlc.narg(requester_id)::uuid IS NULL OR requester_id = sqlc.narg(requester_id)
)
AND (
    sqlc.narg(subnet_id)::uuid IS NULL OR subnet_id = sqlc.narg(subnet_id)
);

-- name: ListIPRequests :many
SELECT id, requester_id, subnet_id, range_id, hostname, notes,
       CASE WHEN requested_address IS NULL THEN '' ELSE host(requested_address)::text END AS requested_address,
       status, allocated_ip_id, reviewed_by, review_notes,
       created_at, updated_at, reviewed_at
FROM ip_requests
WHERE (
    sqlc.arg(status)::text = '' OR status = sqlc.arg(status)
)
AND (
    sqlc.narg(requester_id)::uuid IS NULL OR requester_id = sqlc.narg(requester_id)
)
AND (
    sqlc.narg(subnet_id)::uuid IS NULL OR subnet_id = sqlc.narg(subnet_id)
)
ORDER BY
    CASE WHEN status = 'pending' THEN 0 ELSE 1 END,
    created_at DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountPendingIPRequests :one
SELECT count(*) FROM ip_requests WHERE status = 'pending';

-- name: ApproveIPRequest :one
UPDATE ip_requests
SET status = 'approved',
    allocated_ip_id = sqlc.arg(allocated_ip_id),
    reviewed_by = sqlc.arg(reviewed_by),
    review_notes = sqlc.arg(review_notes),
    reviewed_at = now(),
    updated_at = now()
WHERE id = sqlc.arg(id) AND status = 'pending'
RETURNING id, requester_id, subnet_id, range_id, hostname, notes,
          CASE WHEN requested_address IS NULL THEN '' ELSE host(requested_address)::text END AS requested_address,
          status, allocated_ip_id, reviewed_by, review_notes,
          created_at, updated_at, reviewed_at;

-- name: DenyIPRequest :one
UPDATE ip_requests
SET status = 'denied',
    reviewed_by = sqlc.arg(reviewed_by),
    review_notes = sqlc.arg(review_notes),
    reviewed_at = now(),
    updated_at = now()
WHERE id = sqlc.arg(id) AND status = 'pending'
RETURNING id, requester_id, subnet_id, range_id, hostname, notes,
          CASE WHEN requested_address IS NULL THEN '' ELSE host(requested_address)::text END AS requested_address,
          status, allocated_ip_id, reviewed_by, review_notes,
          created_at, updated_at, reviewed_at;

-- name: DeleteIPRequest :exec
DELETE FROM ip_requests WHERE id = $1 AND status = 'pending';

-- name: ListAllIPRequests :many
SELECT id, requester_id, subnet_id, range_id, hostname, notes,
       CASE WHEN requested_address IS NULL THEN '' ELSE host(requested_address)::text END AS requested_address,
       status, allocated_ip_id, reviewed_by, review_notes,
       created_at, updated_at, reviewed_at
FROM ip_requests
ORDER BY created_at;
