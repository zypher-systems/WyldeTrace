-- name: ListCustomFieldDefs :many
SELECT id, entity_type, key, label, field_type, required, options, sort_order, created_at, updated_at
FROM custom_field_definitions
WHERE entity_type = sqlc.arg(entity_type)
ORDER BY sort_order, key;

-- name: GetCustomFieldDef :one
SELECT id, entity_type, key, label, field_type, required, options, sort_order, created_at, updated_at
FROM custom_field_definitions
WHERE id = $1;

-- name: CreateCustomFieldDef :one
INSERT INTO custom_field_definitions (entity_type, key, label, field_type, required, options, sort_order)
VALUES (
    sqlc.arg(entity_type),
    sqlc.arg(key),
    sqlc.arg(label),
    sqlc.arg(field_type),
    sqlc.arg(required),
    sqlc.arg(options),
    sqlc.arg(sort_order)
)
RETURNING id, entity_type, key, label, field_type, required, options, sort_order, created_at, updated_at;

-- name: UpdateCustomFieldDef :one
UPDATE custom_field_definitions
SET label = sqlc.arg(label),
    field_type = sqlc.arg(field_type),
    required = sqlc.arg(required),
    options = sqlc.arg(options),
    sort_order = sqlc.arg(sort_order),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, entity_type, key, label, field_type, required, options, sort_order, created_at, updated_at;

-- name: DeleteCustomFieldDef :exec
DELETE FROM custom_field_definitions WHERE id = $1;
