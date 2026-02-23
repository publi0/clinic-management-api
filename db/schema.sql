CREATE TABLE IF NOT EXISTS people (
    id UUID PRIMARY KEY,
    person_type TEXT NOT NULL CHECK (person_type IN ('INDIVIDUAL', 'COMPANY')),
    tax_id_type TEXT NOT NULL CHECK (tax_id_type IN ('CPF', 'CNPJ')),
    tax_id_number TEXT NOT NULL,
    legal_name TEXT NOT NULL,
    trade_name TEXT,
    email TEXT,
    phone TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ,
    CHECK (
        (person_type = 'INDIVIDUAL' AND tax_id_type = 'CPF') OR
        (person_type = 'COMPANY' AND tax_id_type = 'CNPJ')
    )
);

CREATE TABLE IF NOT EXISTS clinics (
    id UUID PRIMARY KEY,
    person_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ,
    FOREIGN KEY (person_id) REFERENCES people(id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS dentists (
    id UUID PRIMARY KEY,
    person_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ,
    FOREIGN KEY (person_id) REFERENCES people(id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS clinic_dentists (
    clinic_id UUID NOT NULL,
    dentist_id UUID NOT NULL,
    is_admin BOOLEAN NOT NULL DEFAULT FALSE,
    is_legal_representative BOOLEAN NOT NULL DEFAULT FALSE,
    started_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (clinic_id, dentist_id, started_at),
    FOREIGN KEY (clinic_id) REFERENCES clinics(id) ON DELETE RESTRICT,
    FOREIGN KEY (dentist_id) REFERENCES dentists(id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS bank_accounts (
    id UUID PRIMARY KEY,
    clinic_id UUID NOT NULL,
    bank_code TEXT NOT NULL,
    branch_number TEXT NOT NULL,
    account_number TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ,
    FOREIGN KEY (clinic_id) REFERENCES clinics(id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY,
    email TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_clinic_dentists_active_unique
ON clinic_dentists(clinic_id, dentist_id)
WHERE ended_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_people_tax_id_number ON people(tax_id_number);
CREATE UNIQUE INDEX IF NOT EXISTS idx_people_tax_id_number_active_unique
ON people(tax_id_number)
WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_people_deleted_at ON people(deleted_at);
CREATE INDEX IF NOT EXISTS idx_dentists_person_id ON dentists(person_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_dentists_person_id_active_unique
ON dentists(person_id)
WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_dentists_deleted_at ON dentists(deleted_at);
CREATE INDEX IF NOT EXISTS idx_clinics_person_id ON clinics(person_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_clinics_person_id_active_unique
ON clinics(person_id)
WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_clinics_deleted_at ON clinics(deleted_at);
CREATE INDEX IF NOT EXISTS idx_clinic_dentists_dentist_id ON clinic_dentists(dentist_id);
CREATE INDEX IF NOT EXISTS idx_clinic_dentists_active ON clinic_dentists(clinic_id, dentist_id, ended_at);
CREATE INDEX IF NOT EXISTS idx_bank_accounts_clinic_id ON bank_accounts(clinic_id);
CREATE INDEX IF NOT EXISTS idx_bank_accounts_deleted_at ON bank_accounts(deleted_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_active_unique
ON users(lower(email))
WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at);
