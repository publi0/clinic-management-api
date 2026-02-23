-- name: CreateClinicDentist :one
INSERT INTO clinic_dentists (
    clinic_id,
    dentist_id,
    is_admin,
    is_legal_representative,
    started_at
) VALUES (
    sqlc.arg(clinic_id)::uuid,
    sqlc.arg(dentist_id)::uuid,
    sqlc.arg(is_admin),
    sqlc.arg(is_legal_representative),
    sqlc.arg(started_at)
)
RETURNING *;

-- name: GetActiveClinicDentist :one
SELECT *
FROM clinic_dentists
WHERE clinic_id = sqlc.arg(clinic_id)::uuid
  AND dentist_id = sqlc.arg(dentist_id)::uuid
  AND ended_at IS NULL
ORDER BY started_at DESC
LIMIT 1;

-- name: UpdateClinicDentistRole :one
UPDATE clinic_dentists
SET
    is_admin = COALESCE(sqlc.narg(is_admin), is_admin),
    is_legal_representative = COALESCE(sqlc.narg(is_legal_representative), is_legal_representative),
    updated_at = CURRENT_TIMESTAMP
WHERE clinic_id = sqlc.arg(clinic_id)::uuid
  AND dentist_id = sqlc.arg(dentist_id)::uuid
  AND ended_at IS NULL
RETURNING *;

-- name: EndClinicDentist :execrows
UPDATE clinic_dentists
SET ended_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE clinic_id = sqlc.arg(clinic_id)::uuid
  AND dentist_id = sqlc.arg(dentist_id)::uuid
  AND ended_at IS NULL;

-- name: EndClinicDentistsByDentist :execrows
UPDATE clinic_dentists
SET ended_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE dentist_id = sqlc.arg(dentist_id)::uuid
  AND ended_at IS NULL;

-- name: EndClinicDentistsByClinic :execrows
UPDATE clinic_dentists
SET ended_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE clinic_id = sqlc.arg(clinic_id)::uuid
  AND ended_at IS NULL;

-- name: CountActiveClinicLinksByDentist :one
SELECT COUNT(*)::bigint
FROM clinic_dentists
WHERE dentist_id = sqlc.arg(dentist_id)::uuid
  AND ended_at IS NULL;
