package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/otel"

	"capim-test/internal/db/repository"
	"capim-test/internal/validation"
)

const (
	personTypeCompany    = "COMPANY"
	personTypeIndividual = "INDIVIDUAL"
	taxIDTypeCNPJ        = "CNPJ"
	taxIDTypeCPF         = "CPF"
	serviceTracerName    = "capim-test/internal/service"
)

type Service struct {
	db                *sql.DB
	queries           repository.Querier
	txQuerier         func(tx *sql.Tx) repository.Querier
	jwtSigningKey     []byte
	jwtIssuer         string
	jwtAccessTokenTTL time.Duration
	now               func() time.Time
}

type Option func(*Service)

func New(db *sql.DB, options ...Option) *Service {
	baseQueries := repository.New(db)
	svc := &Service{
		db:                db,
		queries:           baseQueries,
		txQuerier:         func(tx *sql.Tx) repository.Querier { return baseQueries.WithTx(tx) },
		jwtIssuer:         "capim-test-api",
		jwtAccessTokenTTL: 15 * time.Minute,
		now:               time.Now,
	}
	for _, option := range options {
		option(svc)
	}
	return svc
}

func WithAuthConfig(signingKey string, issuer string, accessTokenTTL time.Duration) Option {
	return func(s *Service) {
		s.jwtSigningKey = []byte(strings.TrimSpace(signingKey))
		if strings.TrimSpace(issuer) != "" {
			s.jwtIssuer = strings.TrimSpace(issuer)
		}
		if accessTokenTTL > 0 {
			s.jwtAccessTokenTTL = accessTokenTTL
		}
	}
}

func (s *Service) CreateClinic(ctx context.Context, input CreateClinicInput) (ClinicOutput, error) {
	ctx, span := otel.Tracer(serviceTracerName).Start(ctx, "Service.CreateClinic")
	defer span.End()

	taxID := validation.NormalizeCNPJ(input.TaxIDNumber)
	if !validation.ValidateCNPJ(taxID) {
		return ClinicOutput{}, validationError("invalid CNPJ")
	}
	if strings.TrimSpace(input.LegalName) == "" {
		return ClinicOutput{}, validationError("legal_name is required")
	}
	if input.Email != nil && strings.TrimSpace(*input.Email) != "" && !validation.ValidateEmail(*input.Email) {
		return ClinicOutput{}, validationError("invalid email")
	}
	if len(input.BankAccounts) == 0 {
		return ClinicOutput{}, validationError("bank_accounts must contain at least one account")
	}
	if err := validateBankAccountsInput(input.BankAccounts); err != nil {
		return ClinicOutput{}, err
	}

	personID, err := newUUIDV7()
	if err != nil {
		return ClinicOutput{}, err
	}
	clinicID, err := newUUIDV7()
	if err != nil {
		return ClinicOutput{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ClinicOutput{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	qtx := s.txQuerier(tx)
	person, err := qtx.CreatePerson(ctx, repository.CreatePersonParams{
		ID:          personID,
		PersonType:  personTypeCompany,
		TaxIDType:   taxIDTypeCNPJ,
		TaxIDNumber: taxID,
		LegalName:   strings.TrimSpace(input.LegalName),
		TradeName:   optionalString(input.TradeName),
		Email:       optionalString(input.Email),
		Phone:       optionalString(input.Phone),
	})
	if err != nil {
		return ClinicOutput{}, mapDatabaseError(err)
	}

	clinic, err := qtx.CreateClinic(ctx, repository.CreateClinicParams{ID: clinicID, PersonID: person.ID})
	if err != nil {
		return ClinicOutput{}, mapDatabaseError(err)
	}

	for _, account := range input.BankAccounts {
		bankAccountID, err := newUUIDV7()
		if err != nil {
			return ClinicOutput{}, err
		}

		if _, err := qtx.CreateBankAccount(ctx, repository.CreateBankAccountParams{
			ID:            bankAccountID,
			ClinicID:      clinic.ID,
			BankCode:      strings.TrimSpace(account.BankCode),
			BranchNumber:  strings.TrimSpace(account.BranchNumber),
			AccountNumber: strings.TrimSpace(account.AccountNumber),
		}); err != nil {
			return ClinicOutput{}, mapDatabaseError(err)
		}
	}

	if err := tx.Commit(); err != nil {
		return ClinicOutput{}, fmt.Errorf("commit transaction: %w", err)
	}

	return s.loadClinicSummary(ctx, clinic.ID)
}

func (s *Service) UpdateClinic(ctx context.Context, clinicID string, input UpdateClinicInput) (ClinicOutput, error) {
	ctx, span := otel.Tracer(serviceTracerName).Start(ctx, "Service.UpdateClinic")
	defer span.End()

	if input.LegalName == nil &&
		input.TradeName == nil &&
		input.Email == nil &&
		input.Phone == nil &&
		input.BankAccounts == nil &&
		input.BankAccountIDsToRemove == nil {
		return ClinicOutput{}, validationError("at least one field must be provided")
	}
	if input.LegalName != nil && strings.TrimSpace(*input.LegalName) == "" {
		return ClinicOutput{}, validationError("legal_name cannot be empty")
	}
	if input.Email != nil && strings.TrimSpace(*input.Email) != "" && !validation.ValidateEmail(*input.Email) {
		return ClinicOutput{}, validationError("invalid email")
	}
	if input.BankAccounts != nil {
		if len(*input.BankAccounts) == 0 {
			return ClinicOutput{}, validationError("bank_accounts must contain at least one account when provided")
		}
		if err := validateBankAccountsInput(*input.BankAccounts); err != nil {
			return ClinicOutput{}, err
		}
	}
	if input.BankAccountIDsToRemove != nil {
		if len(*input.BankAccountIDsToRemove) == 0 {
			return ClinicOutput{}, validationError("bank_account_ids_to_remove must contain at least one id when provided")
		}
		for idx, bankAccountID := range *input.BankAccountIDsToRemove {
			parsedID, err := uuid.Parse(strings.TrimSpace(bankAccountID))
			if err != nil || parsedID.Version() != 7 {
				return ClinicOutput{}, validationError(fmt.Sprintf("bank_account_ids_to_remove[%d] must be a UUIDv7", idx))
			}
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ClinicOutput{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	qtx := s.txQuerier(tx)
	clinic, err := qtx.GetClinicByID(ctx, clinicID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ClinicOutput{}, notFoundError("clinic not found")
		}
		return ClinicOutput{}, err
	}

	if input.LegalName != nil || input.TradeName != nil || input.Email != nil || input.Phone != nil {
		if _, err := qtx.UpdatePerson(ctx, repository.UpdatePersonParams{
			ID:        clinic.PersonID,
			LegalName: optionalString(input.LegalName),
			TradeName: optionalString(input.TradeName),
			Email:     optionalString(input.Email),
			Phone:     optionalString(input.Phone),
		}); err != nil {
			return ClinicOutput{}, mapDatabaseError(err)
		}
	}

	if input.BankAccounts != nil {
		for _, account := range *input.BankAccounts {
			bankAccountID, err := newUUIDV7()
			if err != nil {
				return ClinicOutput{}, err
			}
			if _, err := qtx.CreateBankAccount(ctx, repository.CreateBankAccountParams{
				ID:            bankAccountID,
				ClinicID:      clinicID,
				BankCode:      strings.TrimSpace(account.BankCode),
				BranchNumber:  strings.TrimSpace(account.BranchNumber),
				AccountNumber: strings.TrimSpace(account.AccountNumber),
			}); err != nil {
				return ClinicOutput{}, mapDatabaseError(err)
			}
		}
	}
	if input.BankAccountIDsToRemove != nil {
		if err := lockClinicForUpdate(ctx, tx, clinicID); err != nil {
			return ClinicOutput{}, err
		}
		for _, bankAccountID := range *input.BankAccountIDsToRemove {
			affected, err := qtx.DeleteBankAccountByIDAndClinicID(ctx, repository.DeleteBankAccountByIDAndClinicIDParams{
				ID:       strings.TrimSpace(bankAccountID),
				ClinicID: clinicID,
			})
			if err != nil {
				return ClinicOutput{}, mapDatabaseError(err)
			}
			if affected == 0 {
				return ClinicOutput{}, notFoundError("bank account not found")
			}
		}
	}

	activeBankAccounts, err := qtx.ListBankAccountsByClinicID(ctx, clinicID)
	if err != nil {
		return ClinicOutput{}, mapDatabaseError(err)
	}
	if len(activeBankAccounts) == 0 {
		return ClinicOutput{}, validationError("clinic must have at least one active bank account")
	}

	if err := tx.Commit(); err != nil {
		return ClinicOutput{}, fmt.Errorf("commit transaction: %w", err)
	}

	return s.loadClinicSummary(ctx, clinicID)
}

func (s *Service) GetClinic(ctx context.Context, clinicID string) (ClinicDetailsOutput, error) {
	ctx, span := otel.Tracer(serviceTracerName).Start(ctx, "Service.GetClinic")
	defer span.End()

	return s.loadClinicDetails(ctx, clinicID)
}

func (s *Service) ListClinicsWithCursor(ctx context.Context, limit int, cursor *string) ([]ClinicOutput, *string, error) {
	ctx, span := otel.Tracer(serviceTracerName).Start(ctx, "Service.ListClinicsWithCursor")
	defer span.End()

	pageLimit := normalizeCursorLimit(limit)
	queryLimit := int32(pageLimit + 1)

	afterID := uuid.NullUUID{}
	if cursor != nil {
		parsedAfterID, err := uuid.Parse(*cursor)
		if err != nil {
			return nil, nil, validationError("invalid cursor")
		}
		afterID.UUID = parsedAfterID
		afterID.Valid = true
	}

	rows, err := s.queries.ListClinicDetailsCursor(ctx, repository.ListClinicDetailsCursorParams{
		AfterID:   afterID,
		PageLimit: queryLimit,
	})
	if err != nil {
		return nil, nil, err
	}

	hasNext := len(rows) > pageLimit
	if hasNext {
		rows = rows[:pageLimit]
	}

	clinicIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		clinicIDs = append(clinicIDs, row.ClinicID)
	}

	dentistIDsByClinic, err := s.loadClinicDentistIDsByClinicIDs(ctx, clinicIDs)
	if err != nil {
		return nil, nil, err
	}

	clinics := make([]ClinicOutput, 0, len(rows))
	for _, row := range rows {
		clinics = append(clinics, mapClinicSummary(
			row.ClinicID,
			row.PersonID,
			row.LegalName,
			row.TradeName,
			row.TaxIDNumber,
			row.Email,
			row.Phone,
			dentistIDsByClinic[row.ClinicID],
		))
	}

	var nextCursor *string
	if hasNext && len(rows) > 0 {
		cursorValue := rows[len(rows)-1].ClinicID
		nextCursor = &cursorValue
	}

	return clinics, nextCursor, nil
}

func (s *Service) DeleteClinic(ctx context.Context, clinicID string) error {
	ctx, span := otel.Tracer(serviceTracerName).Start(ctx, "Service.DeleteClinic")
	defer span.End()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	qtx := s.txQuerier(tx)
	clinic, err := qtx.GetClinicByID(ctx, clinicID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return notFoundError("clinic not found")
		}
		return err
	}

	if _, err := qtx.EndClinicDentistsByClinic(ctx, clinicID); err != nil {
		return mapDatabaseError(err)
	}
	if _, err := qtx.DeleteClinic(ctx, clinicID); err != nil {
		return mapDatabaseError(err)
	}
	if _, err := qtx.DeletePerson(ctx, clinic.PersonID); err != nil {
		return mapDatabaseError(err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func (s *Service) CreateOrAttachDentist(ctx context.Context, clinicID string, input CreateDentistInput) (ClinicDentistOutput, bool, error) {
	ctx, span := otel.Tracer(serviceTracerName).Start(ctx, "Service.CreateOrAttachDentist")
	defer span.End()

	taxID := validation.NormalizeCPF(input.TaxIDNumber)
	if !validation.ValidateCPF(taxID) {
		return ClinicDentistOutput{}, false, validationError("invalid CPF")
	}
	if strings.TrimSpace(input.LegalName) == "" {
		return ClinicDentistOutput{}, false, validationError("legal_name is required")
	}
	if input.Email != nil && strings.TrimSpace(*input.Email) != "" && !validation.ValidateEmail(*input.Email) {
		return ClinicDentistOutput{}, false, validationError("invalid email")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ClinicDentistOutput{}, false, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	qtx := s.txQuerier(tx)
	if _, err := qtx.GetClinicByID(ctx, clinicID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ClinicDentistOutput{}, false, notFoundError("clinic not found")
		}
		return ClinicDentistOutput{}, false, err
	}

	var person repository.Person
	var dentist repository.Dentist

	person, err = qtx.GetPersonByTaxID(ctx, taxID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return ClinicDentistOutput{}, false, err
		}

		personID, err := newUUIDV7()
		if err != nil {
			return ClinicDentistOutput{}, false, err
		}

		person, err = qtx.CreatePerson(ctx, repository.CreatePersonParams{
			ID:          personID,
			PersonType:  personTypeIndividual,
			TaxIDType:   taxIDTypeCPF,
			TaxIDNumber: taxID,
			LegalName:   strings.TrimSpace(input.LegalName),
			Email:       optionalString(input.Email),
			Phone:       optionalString(input.Phone),
		})
		if err != nil {
			if isUniqueConstraintError(err) {
				// Another concurrent request created the person first; continue using the existing row.
				person, err = qtx.GetPersonByTaxID(ctx, taxID)
				if err != nil {
					return ClinicDentistOutput{}, false, mapDatabaseError(err)
				}
			} else {
				return ClinicDentistOutput{}, false, mapDatabaseError(err)
			}
		}
	}
	if person.PersonType != personTypeIndividual {
		return ClinicDentistOutput{}, false, conflictError("tax_id is linked to a company person")
	}

	person, err = qtx.UpdatePerson(ctx, repository.UpdatePersonParams{
		ID:        person.ID,
		LegalName: optionalString(new(strings.TrimSpace(input.LegalName))),
		Email:     optionalString(input.Email),
		Phone:     optionalString(input.Phone),
	})
	if err != nil {
		return ClinicDentistOutput{}, false, mapDatabaseError(err)
	}

	dentist, err = qtx.GetDentistByPersonID(ctx, person.ID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return ClinicDentistOutput{}, false, err
		}

		dentistID, err := newUUIDV7()
		if err != nil {
			return ClinicDentistOutput{}, false, err
		}
		dentist, err = qtx.CreateDentist(ctx, repository.CreateDentistParams{ID: dentistID, PersonID: person.ID})
		if err != nil {
			if isUniqueConstraintError(err) {
				// Another concurrent request created the dentist first; continue with the existing row.
				dentist, err = qtx.GetDentistByPersonID(ctx, person.ID)
				if err != nil {
					return ClinicDentistOutput{}, false, mapDatabaseError(err)
				}
			} else {
				return ClinicDentistOutput{}, false, mapDatabaseError(err)
			}
		}
	}

	created := false
	relation, err := qtx.GetActiveClinicDentist(ctx, repository.GetActiveClinicDentistParams{ClinicID: clinicID, DentistID: dentist.ID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			relation, err = qtx.CreateClinicDentist(ctx, repository.CreateClinicDentistParams{
				ClinicID:              clinicID,
				DentistID:             dentist.ID,
				IsAdmin:               input.IsAdmin,
				IsLegalRepresentative: input.IsLegalRepresentative,
				StartedAt:             time.Now().UTC(),
			})
			if err != nil {
				if isUniqueConstraintError(err) {
					// Another concurrent request created the active link first.
					relation, err = qtx.GetActiveClinicDentist(ctx, repository.GetActiveClinicDentistParams{ClinicID: clinicID, DentistID: dentist.ID})
					if err != nil {
						return ClinicDentistOutput{}, false, mapDatabaseError(err)
					}
					relation, err = qtx.UpdateClinicDentistRole(ctx, repository.UpdateClinicDentistRoleParams{
						ClinicID:              clinicID,
						DentistID:             dentist.ID,
						IsAdmin:               sql.NullBool{Bool: input.IsAdmin, Valid: true},
						IsLegalRepresentative: sql.NullBool{Bool: input.IsLegalRepresentative, Valid: true},
					})
					if err != nil {
						return ClinicDentistOutput{}, false, mapDatabaseError(err)
					}
				} else {
					return ClinicDentistOutput{}, false, mapDatabaseError(err)
				}
			} else {
				created = true
			}
		} else {
			return ClinicDentistOutput{}, false, mapDatabaseError(err)
		}
	} else {
		relation, err = qtx.UpdateClinicDentistRole(ctx, repository.UpdateClinicDentistRoleParams{
			ClinicID:              clinicID,
			DentistID:             dentist.ID,
			IsAdmin:               sql.NullBool{Bool: input.IsAdmin, Valid: true},
			IsLegalRepresentative: sql.NullBool{Bool: input.IsLegalRepresentative, Valid: true},
		})
		if err != nil {
			return ClinicDentistOutput{}, false, mapDatabaseError(err)
		}
	}

	if err := tx.Commit(); err != nil {
		return ClinicDentistOutput{}, false, fmt.Errorf("commit transaction: %w", err)
	}

	return ClinicDentistOutput{
		DentistOutput: DentistOutput{
			ID:          dentist.ID,
			PersonID:    person.ID,
			LegalName:   person.LegalName,
			TaxIDNumber: person.TaxIDNumber,
			Email:       nullToPointer(person.Email),
			Phone:       nullToPointer(person.Phone),
		},
		IsAdmin:               relation.IsAdmin,
		IsLegalRepresentative: relation.IsLegalRepresentative,
		StartedAt:             relation.StartedAt,
	}, created, nil
}

func (s *Service) ListClinicDentistsWithCursor(ctx context.Context, clinicID string, limit int, cursor *string) ([]ClinicDentistOutput, *string, error) {
	ctx, span := otel.Tracer(serviceTracerName).Start(ctx, "Service.ListClinicDentistsWithCursor")
	defer span.End()

	if _, err := s.queries.GetClinicByID(ctx, clinicID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, notFoundError("clinic not found")
		}
		return nil, nil, err
	}

	pageLimit := normalizeCursorLimit(limit)
	queryLimit := int32(pageLimit + 1)

	afterDentistID := uuid.NullUUID{}
	if cursor != nil {
		parsedAfterID, err := uuid.Parse(*cursor)
		if err != nil {
			return nil, nil, validationError("invalid cursor")
		}
		afterDentistID.UUID = parsedAfterID
		afterDentistID.Valid = true
	}

	rows, err := s.queries.ListDentistsByClinicIDCursor(ctx, repository.ListDentistsByClinicIDCursorParams{
		ClinicID:       clinicID,
		AfterDentistID: afterDentistID,
		PageLimit:      queryLimit,
	})
	if err != nil {
		return nil, nil, err
	}

	hasNext := len(rows) > pageLimit
	if hasNext {
		rows = rows[:pageLimit]
	}

	output := make([]ClinicDentistOutput, 0, len(rows))
	for _, row := range rows {
		output = append(output, mapDentistCursorRow(row))
	}

	var nextCursor *string
	if hasNext && len(rows) > 0 {
		cursorValue := rows[len(rows)-1].DentistID
		nextCursor = &cursorValue
	}

	return output, nextCursor, nil
}

func (s *Service) UpdateClinicDentistRole(ctx context.Context, clinicID string, dentistID string, input UpdateClinicDentistRoleInput) (ClinicDentistOutput, error) {
	ctx, span := otel.Tracer(serviceTracerName).Start(ctx, "Service.UpdateClinicDentistRole")
	defer span.End()

	if input.IsAdmin == nil && input.IsLegalRepresentative == nil {
		return ClinicDentistOutput{}, validationError("at least one role field must be provided")
	}

	relation, err := s.queries.UpdateClinicDentistRole(ctx, repository.UpdateClinicDentistRoleParams{
		ClinicID:              clinicID,
		DentistID:             dentistID,
		IsAdmin:               optionalBool(input.IsAdmin),
		IsLegalRepresentative: optionalBool(input.IsLegalRepresentative),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ClinicDentistOutput{}, notFoundError("clinic dentist active link not found")
		}
		return ClinicDentistOutput{}, mapDatabaseError(err)
	}

	details, err := s.queries.GetDentistDetailsByID(ctx, dentistID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ClinicDentistOutput{}, notFoundError("dentist not found")
		}
		return ClinicDentistOutput{}, err
	}

	return ClinicDentistOutput{
		DentistOutput: DentistOutput{
			ID:          details.DentistID,
			PersonID:    details.PersonID,
			LegalName:   details.LegalName,
			TaxIDNumber: details.TaxIDNumber,
			Email:       nullToPointer(details.Email),
			Phone:       nullToPointer(details.Phone),
		},
		IsAdmin:               relation.IsAdmin,
		IsLegalRepresentative: relation.IsLegalRepresentative,
		StartedAt:             relation.StartedAt,
	}, nil
}

func (s *Service) UnlinkDentistFromClinic(ctx context.Context, clinicID string, dentistID string) error {
	ctx, span := otel.Tracer(serviceTracerName).Start(ctx, "Service.UnlinkDentistFromClinic")
	defer span.End()

	if _, err := s.queries.GetActiveClinicDentist(ctx, repository.GetActiveClinicDentistParams{
		ClinicID:  clinicID,
		DentistID: dentistID,
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return notFoundError("clinic dentist active link not found")
		}
		return mapDatabaseError(err)
	}

	activeLinks, err := s.queries.CountActiveClinicLinksByDentist(ctx, dentistID)
	if err != nil {
		return mapDatabaseError(err)
	}
	if activeLinks <= 1 {
		return conflictError("cannot unlink dentist from the last active clinic")
	}

	affected, err := s.queries.EndClinicDentist(ctx, repository.EndClinicDentistParams{ClinicID: clinicID, DentistID: dentistID})
	if err != nil {
		return mapDatabaseError(err)
	}
	if affected == 0 {
		return notFoundError("clinic dentist active link not found")
	}
	return nil
}

func (s *Service) UpdateDentist(ctx context.Context, dentistID string, input UpdateDentistInput) (DentistOutput, error) {
	ctx, span := otel.Tracer(serviceTracerName).Start(ctx, "Service.UpdateDentist")
	defer span.End()

	if input.LegalName == nil && input.Email == nil && input.Phone == nil {
		return DentistOutput{}, validationError("at least one field must be provided")
	}
	if input.LegalName != nil && strings.TrimSpace(*input.LegalName) == "" {
		return DentistOutput{}, validationError("legal_name cannot be empty")
	}
	if input.Email != nil && strings.TrimSpace(*input.Email) != "" && !validation.ValidateEmail(*input.Email) {
		return DentistOutput{}, validationError("invalid email")
	}

	dentist, err := s.queries.GetDentistByID(ctx, dentistID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DentistOutput{}, notFoundError("dentist not found")
		}
		return DentistOutput{}, err
	}

	person, err := s.queries.UpdatePerson(ctx, repository.UpdatePersonParams{
		ID:        dentist.PersonID,
		LegalName: optionalString(input.LegalName),
		Email:     optionalString(input.Email),
		Phone:     optionalString(input.Phone),
	})
	if err != nil {
		return DentistOutput{}, mapDatabaseError(err)
	}

	return DentistOutput{
		ID:          dentist.ID,
		PersonID:    person.ID,
		LegalName:   person.LegalName,
		TaxIDNumber: person.TaxIDNumber,
		Email:       nullToPointer(person.Email),
		Phone:       nullToPointer(person.Phone),
	}, nil
}

func (s *Service) DeleteDentist(ctx context.Context, dentistID string) error {
	ctx, span := otel.Tracer(serviceTracerName).Start(ctx, "Service.DeleteDentist")
	defer span.End()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	qtx := s.txQuerier(tx)
	dentist, err := qtx.GetDentistByID(ctx, dentistID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return notFoundError("dentist not found")
		}
		return err
	}

	if _, err := qtx.EndClinicDentistsByDentist(ctx, dentistID); err != nil {
		return mapDatabaseError(err)
	}
	if _, err := qtx.DeleteDentist(ctx, dentistID); err != nil {
		return mapDatabaseError(err)
	}
	if _, err := qtx.DeletePerson(ctx, dentist.PersonID); err != nil {
		return mapDatabaseError(err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func (s *Service) loadClinicSummary(ctx context.Context, clinicID string) (ClinicOutput, error) {
	row, err := s.queries.GetClinicDetails(ctx, clinicID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ClinicOutput{}, notFoundError("clinic not found")
		}
		return ClinicOutput{}, err
	}

	dentists, err := s.queries.ListDentistsByClinicID(ctx, clinicID)
	if err != nil {
		return ClinicOutput{}, err
	}

	return mapClinicSummary(
		row.ClinicID,
		row.PersonID,
		row.LegalName,
		row.TradeName,
		row.TaxIDNumber,
		row.Email,
		row.Phone,
		mapDentistIDs(dentists),
	), nil
}

func (s *Service) loadClinicDetails(ctx context.Context, clinicID string) (ClinicDetailsOutput, error) {
	row, err := s.queries.GetClinicDetails(ctx, clinicID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ClinicDetailsOutput{}, notFoundError("clinic not found")
		}
		return ClinicDetailsOutput{}, err
	}

	dentists, err := s.queries.ListDentistsByClinicID(ctx, clinicID)
	if err != nil {
		return ClinicDetailsOutput{}, err
	}
	bankAccounts, err := s.queries.ListBankAccountsByClinicID(ctx, clinicID)
	if err != nil {
		return ClinicDetailsOutput{}, err
	}

	return mapClinicDetails(
		row.ClinicID,
		row.PersonID,
		row.LegalName,
		row.TradeName,
		row.TaxIDNumber,
		row.Email,
		row.Phone,
		mapDentistIDs(dentists),
		bankAccounts,
	), nil
}

func (s *Service) loadClinicDentistIDsByClinicIDs(ctx context.Context, clinicIDs []string) (map[string][]string, error) {
	dentistIDsByClinic := make(map[string][]string, len(clinicIDs))
	if len(clinicIDs) == 0 {
		return dentistIDsByClinic, nil
	}

	dentistRows, err := s.queries.ListDentistsByClinicIDs(ctx, clinicIDs)
	if err != nil {
		return nil, err
	}
	for _, row := range dentistRows {
		dentistIDsByClinic[row.ClinicID] = append(dentistIDsByClinic[row.ClinicID], row.DentistID)
	}

	return dentistIDsByClinic, nil
}

func mapClinicSummary(
	clinicID string,
	personID string,
	legalName string,
	tradeName sql.NullString,
	taxIDNumber string,
	email sql.NullString,
	phone sql.NullString,
	dentistIDs []string,
) ClinicOutput {
	if dentistIDs == nil {
		dentistIDs = make([]string, 0)
	}

	return ClinicOutput{
		ID:          clinicID,
		PersonID:    personID,
		LegalName:   legalName,
		TradeName:   nullToPointer(tradeName),
		TaxIDNumber: taxIDNumber,
		Email:       nullToPointer(email),
		Phone:       nullToPointer(phone),
		DentistIDs:  dentistIDs,
	}
}

func mapClinicDetails(
	clinicID string,
	personID string,
	legalName string,
	tradeName sql.NullString,
	taxIDNumber string,
	email sql.NullString,
	phone sql.NullString,
	dentistIDs []string,
	bankAccounts []repository.BankAccount,
) ClinicDetailsOutput {
	return ClinicDetailsOutput{
		ClinicOutput: mapClinicSummary(
			clinicID,
			personID,
			legalName,
			tradeName,
			taxIDNumber,
			email,
			phone,
			dentistIDs,
		),
		BankAccounts: mapBankAccounts(bankAccounts),
	}
}

func mapDentistIDs(rows []repository.ListDentistsByClinicIDRow) []string {
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.DentistID)
	}
	return ids
}

func mapBankAccounts(rows []repository.BankAccount) []BankAccountOutput {
	accounts := make([]BankAccountOutput, 0, len(rows))
	for _, row := range rows {
		accounts = append(accounts, BankAccountOutput{
			ID:            row.ID,
			BankCode:      row.BankCode,
			BranchNumber:  row.BranchNumber,
			AccountNumber: row.AccountNumber,
		})
	}
	return accounts
}

func mapDentistCursorRow(row repository.ListDentistsByClinicIDCursorRow) ClinicDentistOutput {
	return mapClinicDentistSummary(
		row.DentistID,
		row.PersonID,
		row.LegalName,
		row.TaxIDNumber,
		row.Email,
		row.Phone,
		row.IsAdmin,
		row.IsLegalRepresentative,
		row.StartedAt,
	)
}

func mapClinicDentistSummary(
	dentistID string,
	personID string,
	legalName string,
	taxIDNumber string,
	email sql.NullString,
	phone sql.NullString,
	isAdmin bool,
	isLegalRepresentative bool,
	startedAt time.Time,
) ClinicDentistOutput {
	return ClinicDentistOutput{
		DentistOutput: DentistOutput{
			ID:          dentistID,
			PersonID:    personID,
			LegalName:   legalName,
			TaxIDNumber: taxIDNumber,
			Email:       nullToPointer(email),
			Phone:       nullToPointer(phone),
		},
		IsAdmin:               isAdmin,
		IsLegalRepresentative: isLegalRepresentative,
		StartedAt:             startedAt,
	}
}

func validateBankAccountInput(input BankAccountInput) error {
	if strings.TrimSpace(input.BankCode) == "" {
		return fmt.Errorf("bank_code is required")
	}
	if strings.TrimSpace(input.BranchNumber) == "" {
		return fmt.Errorf("branch_number is required")
	}
	if strings.TrimSpace(input.AccountNumber) == "" {
		return fmt.Errorf("account_number is required")
	}
	return nil
}

func validateBankAccountsInput(accounts []BankAccountInput) error {
	for idx, account := range accounts {
		if err := validateBankAccountInput(account); err != nil {
			return validationError(fmt.Sprintf("bank_accounts[%d]: %s", idx, err.Error()))
		}
	}
	return nil
}

func lockClinicForUpdate(ctx context.Context, tx *sql.Tx, clinicID string) error {
	const lockClinicSQL = `
SELECT id
FROM clinics
WHERE id = $1 AND deleted_at IS NULL
FOR UPDATE
`
	var lockedClinicID string
	if err := tx.QueryRowContext(ctx, lockClinicSQL, clinicID).Scan(&lockedClinicID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return notFoundError("clinic not found")
		}
		return mapDatabaseError(err)
	}
	return nil
}

func optionalString(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: trimmed, Valid: true}
}

func optionalBool(value *bool) sql.NullBool {
	if value == nil {
		return sql.NullBool{}
	}
	return sql.NullBool{Bool: *value, Valid: true}
}

func nullToPointer(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	v := value.String
	return &v
}

func newUUIDV7() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("generate uuidv7: %w", err)
	}
	return id.String(), nil
}

func mapDatabaseError(err error) error {
	if isUniqueConstraintError(err) {
		return conflictError("resource already exists")
	}
	if isForeignKeyConstraintError(err) {
		return validationError("invalid relationship reference")
	}
	return err
}

func isUniqueConstraintError(err error) bool {
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		return pgErr.Code == "23505"
	}
	return strings.Contains(strings.ToLower(err.Error()), "duplicate key value violates unique constraint")
}

func isForeignKeyConstraintError(err error) bool {
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		return pgErr.Code == "23503"
	}
	return strings.Contains(strings.ToLower(err.Error()), "violates foreign key constraint")
}

func normalizeCursorLimit(limit int) int {
	const (
		defaultLimit = 20
		maxLimit     = 100
	)

	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}
