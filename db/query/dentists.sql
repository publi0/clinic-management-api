-- name: CreateDentist :one
INSERT INTO dentists (id, person_id)
VALUES (sqlc.arg(id)::uuid, sqlc.arg(person_id)::uuid)
RETURNING *;

-- name: GetDentistByID :one
SELECT *
FROM dentists
WHERE id = sqlc.arg(id)::uuid
  AND deleted_at IS NULL
LIMIT 1;

-- name: GetDentistByPersonID :one
SELECT *
FROM dentists
WHERE person_id = sqlc.arg(person_id)::uuid
  AND deleted_at IS NULL
LIMIT 1;

-- name: DeleteDentist :execrows
UPDATE dentists
SET deleted_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id)::uuid
  AND deleted_at IS NULL;

-- name: GetDentistDetailsByID :one
SELECT
    d.id AS dentist_id,
    d.person_id,
    p.legal_name,
    p.tax_id_number,
    p.email,
    p.phone
FROM dentists d
JOIN people p ON p.id = d.person_id
WHERE d.id = sqlc.arg(id)::uuid
  AND d.deleted_at IS NULL
  AND p.deleted_at IS NULL
LIMIT 1;

-- name: ListDentistsByClinicID :many
SELECT
    d.id AS dentist_id,
    d.person_id,
    p.legal_name,
    p.tax_id_number,
    p.email,
    p.phone,
    cd.is_admin,
    cd.is_legal_representative,
    cd.started_at,
    cd.ended_at
FROM clinic_dentists cd
JOIN dentists d ON d.id = cd.dentist_id
JOIN people p ON p.id = d.person_id
JOIN clinics c ON c.id = cd.clinic_id
WHERE cd.clinic_id = sqlc.arg(clinic_id)::uuid
  AND cd.ended_at IS NULL
  AND d.deleted_at IS NULL
  AND p.deleted_at IS NULL
  AND c.deleted_at IS NULL
ORDER BY d.id;

-- name: ListDentistsByClinicIDCursor :many
SELECT
    d.id AS dentist_id,
    d.person_id,
    p.legal_name,
    p.tax_id_number,
    p.email,
    p.phone,
    cd.is_admin,
    cd.is_legal_representative,
    cd.started_at,
    cd.ended_at
FROM clinic_dentists cd
JOIN dentists d ON d.id = cd.dentist_id
JOIN people p ON p.id = d.person_id
JOIN clinics c ON c.id = cd.clinic_id
WHERE cd.clinic_id = sqlc.arg(clinic_id)::uuid
  AND cd.ended_at IS NULL
  AND d.deleted_at IS NULL
  AND p.deleted_at IS NULL
  AND c.deleted_at IS NULL
  AND (sqlc.narg(after_dentist_id)::uuid IS NULL OR d.id > sqlc.narg(after_dentist_id)::uuid)
ORDER BY d.id
LIMIT sqlc.arg(page_limit);

-- name: ListDentistsByClinicIDs :many
SELECT
    cd.clinic_id,
    d.id AS dentist_id,
    d.person_id,
    p.legal_name,
    p.tax_id_number,
    p.email,
    p.phone,
    cd.is_admin,
    cd.is_legal_representative,
    cd.started_at,
    cd.ended_at
FROM clinic_dentists cd
JOIN dentists d ON d.id = cd.dentist_id
JOIN people p ON p.id = d.person_id
JOIN clinics c ON c.id = cd.clinic_id
WHERE cd.clinic_id = ANY(sqlc.arg(clinic_ids)::uuid[])
  AND cd.ended_at IS NULL
  AND d.deleted_at IS NULL
  AND p.deleted_at IS NULL
  AND c.deleted_at IS NULL
ORDER BY cd.clinic_id, d.id;
