-- name: CreatePerson :one
INSERT INTO people (
    id,
    person_type,
    tax_id_type,
    tax_id_number,
    legal_name,
    trade_name,
    email,
    phone
) VALUES (
    sqlc.arg(id)::uuid,
    sqlc.arg(person_type),
    sqlc.arg(tax_id_type),
    sqlc.arg(tax_id_number),
    sqlc.arg(legal_name),
    sqlc.arg(trade_name),
    sqlc.arg(email),
    sqlc.arg(phone)
)
RETURNING *;

-- name: GetPersonByTaxID :one
SELECT *
FROM people
WHERE tax_id_number = sqlc.arg(tax_id_number)
  AND deleted_at IS NULL
LIMIT 1;

-- name: UpdatePerson :one
UPDATE people
SET
    legal_name = COALESCE(sqlc.narg(legal_name), legal_name),
    trade_name = COALESCE(sqlc.narg(trade_name), trade_name),
    email = COALESCE(sqlc.narg(email), email),
    phone = COALESCE(sqlc.narg(phone), phone),
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id)::uuid
  AND deleted_at IS NULL
RETURNING *;

-- name: DeletePerson :execrows
UPDATE people
SET deleted_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id)::uuid
  AND deleted_at IS NULL;
