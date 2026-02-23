-- name: CreateBankAccount :one
INSERT INTO bank_accounts (
    id,
    clinic_id,
    bank_code,
    branch_number,
    account_number
) VALUES (
    sqlc.arg(id)::uuid,
    sqlc.arg(clinic_id)::uuid,
    sqlc.arg(bank_code),
    sqlc.arg(branch_number),
    sqlc.arg(account_number)
)
RETURNING *;

-- name: ListBankAccountsByClinicID :many
SELECT *
FROM bank_accounts
WHERE clinic_id = sqlc.arg(clinic_id)::uuid
  AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: GetBankAccountByIDAndClinicID :one
SELECT *
FROM bank_accounts
WHERE id = sqlc.arg(id)::uuid
  AND clinic_id = sqlc.arg(clinic_id)::uuid
  AND deleted_at IS NULL
LIMIT 1;

-- name: DeleteBankAccountByIDAndClinicID :execrows
UPDATE bank_accounts
SET deleted_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id)::uuid
  AND clinic_id = sqlc.arg(clinic_id)::uuid
  AND deleted_at IS NULL;
