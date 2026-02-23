-- name: CreateClinic :one
INSERT INTO clinics (id, person_id)
VALUES (sqlc.arg(id)::uuid, sqlc.arg(person_id)::uuid)
RETURNING *;

-- name: GetClinicByID :one
SELECT *
FROM clinics
WHERE id = sqlc.arg(id)::uuid
  AND deleted_at IS NULL
LIMIT 1;

-- name: DeleteClinic :execrows
UPDATE clinics
SET deleted_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id)::uuid
  AND deleted_at IS NULL;

-- name: GetClinicDetails :one
SELECT
    c.id AS clinic_id,
    c.person_id,
    p.legal_name,
    p.trade_name,
    p.tax_id_number,
    p.email,
    p.phone
FROM clinics c
JOIN people p ON p.id = c.person_id
WHERE c.id = sqlc.arg(id)::uuid
  AND c.deleted_at IS NULL
  AND p.deleted_at IS NULL
LIMIT 1;

-- name: ListClinicDetailsCursor :many
SELECT
    c.id AS clinic_id,
    c.person_id,
    p.legal_name,
    p.trade_name,
    p.tax_id_number,
    p.email,
    p.phone
FROM clinics c
JOIN people p ON p.id = c.person_id
WHERE c.deleted_at IS NULL
  AND p.deleted_at IS NULL
  AND (sqlc.narg(after_id)::uuid IS NULL OR c.id > sqlc.narg(after_id)::uuid)
ORDER BY c.id
LIMIT sqlc.arg(page_limit);
