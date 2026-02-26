package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"capim-test/internal/db/repository"
)

type mockQuerier struct {
	repository.Querier
	getUserByEmailFn             func(ctx context.Context, email string) (repository.User, error)
	createUserFn                 func(ctx context.Context, arg repository.CreateUserParams) (repository.User, error)
	getClinicByIDFn              func(ctx context.Context, id string) (repository.Clinic, error)
	lockClinicForUpdateFn        func(ctx context.Context, id string) (string, error)
	endClinicDentistsByClinicFn  func(ctx context.Context, clinicID string) (int64, error)
	deleteBankAccountsByClinicFn func(ctx context.Context, clinicID string) (int64, error)
	deleteClinicFn               func(ctx context.Context, id string) (int64, error)
	deletePersonFn               func(ctx context.Context, id string) (int64, error)
}

func (m mockQuerier) GetUserByEmail(ctx context.Context, email string) (repository.User, error) {
	if m.getUserByEmailFn != nil {
		return m.getUserByEmailFn(ctx, email)
	}
	return repository.User{}, sql.ErrNoRows
}

func (m mockQuerier) CreateUser(ctx context.Context, arg repository.CreateUserParams) (repository.User, error) {
	if m.createUserFn != nil {
		return m.createUserFn(ctx, arg)
	}
	return repository.User{ID: arg.ID, Email: arg.Email, PasswordHash: arg.PasswordHash}, nil
}

func (m mockQuerier) GetClinicByID(ctx context.Context, id string) (repository.Clinic, error) {
	if m.getClinicByIDFn != nil {
		return m.getClinicByIDFn(ctx, id)
	}
	return repository.Clinic{}, sql.ErrNoRows
}

func (m mockQuerier) LockClinicForUpdate(ctx context.Context, id string) (string, error) {
	if m.lockClinicForUpdateFn != nil {
		return m.lockClinicForUpdateFn(ctx, id)
	}
	return id, nil
}

func (m mockQuerier) EndClinicDentistsByClinic(ctx context.Context, clinicID string) (int64, error) {
	if m.endClinicDentistsByClinicFn != nil {
		return m.endClinicDentistsByClinicFn(ctx, clinicID)
	}
	return 1, nil
}

func (m mockQuerier) DeleteBankAccountsByClinicID(ctx context.Context, clinicID string) (int64, error) {
	if m.deleteBankAccountsByClinicFn != nil {
		return m.deleteBankAccountsByClinicFn(ctx, clinicID)
	}
	return 1, nil
}

func (m mockQuerier) DeleteClinic(ctx context.Context, id string) (int64, error) {
	if m.deleteClinicFn != nil {
		return m.deleteClinicFn(ctx, id)
	}
	return 1, nil
}

func (m mockQuerier) DeletePerson(ctx context.Context, id string) (int64, error) {
	if m.deletePersonFn != nil {
		return m.deletePersonFn(ctx, id)
	}
	return 1, nil
}

func newAuthServiceForTest(q repository.Querier) *Service {
	return &Service{
		queries:           q,
		jwtSigningKey:     []byte("test-secret-key"),
		jwtIssuer:         "capim-test",
		jwtAccessTokenTTL: 15 * time.Minute,
		now:               time.Now,
	}
}

func TestCreateClinicInvalidCNPJ(t *testing.T) {
	svc := &Service{}

	_, err := svc.CreateClinic(context.Background(), CreateClinicInput{
		TaxIDNumber: "123",
		LegalName:   "Invalid Co",
		BankAccounts: []BankAccountInput{{
			BankCode:      "001",
			BranchNumber:  "1234",
			AccountNumber: "998877",
		}},
	})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation, got: %v", err)
	}
}

func TestUpdateClinicInvalidBankAccountIDToRemove(t *testing.T) {
	svc := &Service{}
	invalid := []string{"not-a-uuid-v7"}

	_, err := svc.UpdateClinic(context.Background(), "019f3329-a5a8-72ec-a95b-6e554247f442", UpdateClinicInput{
		BankAccountIDsToRemove: &invalid,
	})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation, got: %v", err)
	}
}

func TestValidateMaxLengthCountsUnicodeCharacters(t *testing.T) {
	if err := validateMaxLength("legal_name", strings.Repeat("á", 255), 255); err != nil {
		t.Fatalf("expected multibyte input within character limit to pass, got: %v", err)
	}

	err := validateMaxLength("legal_name", strings.Repeat("á", 256), 255)
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation for input over character limit, got: %v", err)
	}
}

func TestValidateBankAccountInputCountsUnicodeCharacters(t *testing.T) {
	valid := BankAccountInput{
		BankCode:      strings.Repeat("ç", maxBankFieldLength),
		BranchNumber:  "1234",
		AccountNumber: "998877",
	}
	if err := validateBankAccountInput(valid); err != nil {
		t.Fatalf("expected multibyte bank code within character limit to pass, got: %v", err)
	}

	invalid := BankAccountInput{
		BankCode:      strings.Repeat("ç", maxBankFieldLength+1),
		BranchNumber:  "1234",
		AccountNumber: "998877",
	}
	if err := validateBankAccountInput(invalid); err == nil {
		t.Fatalf("expected validation error for bank code over character limit")
	}
}

func TestCreateClinicRejectsOversizedUnicodeBeforeDB(t *testing.T) {
	svc := &Service{}

	_, err := svc.CreateClinic(context.Background(), CreateClinicInput{
		TaxIDNumber: "43542338000150",
		LegalName:   strings.Repeat("á", maxLegalNameLength+1),
		BankAccounts: []BankAccountInput{{
			BankCode:      "001",
			BranchNumber:  "1234",
			AccountNumber: "998877",
		}},
	})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation for oversized legal_name, got: %v", err)
	}
}

func TestUpdateClinicRejectsOversizedUnicodeBeforeDB(t *testing.T) {
	svc := &Service{}
	tooLong := strings.Repeat("ç", maxTradeNameLength+1)

	_, err := svc.UpdateClinic(context.Background(), "019f3329-a5a8-72ec-a95b-6e554247f442", UpdateClinicInput{
		TradeName: &tooLong,
	})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation for oversized trade_name, got: %v", err)
	}
}

func TestDeleteClinicLocksClinicBeforeDeletingBankAccounts(t *testing.T) {
	clinicID := "019f3329-a5a8-72ec-a95b-6e554247f442"
	personID := "019f3329-a5a8-72ec-a95b-6e554247f443"
	calls := make([]string, 0, 6)

	q := mockQuerier{
		getClinicByIDFn: func(ctx context.Context, id string) (repository.Clinic, error) {
			calls = append(calls, "GetClinicByID")
			return repository.Clinic{ID: id, PersonID: personID}, nil
		},
		lockClinicForUpdateFn: func(ctx context.Context, id string) (string, error) {
			calls = append(calls, "LockClinicForUpdate")
			return id, nil
		},
		endClinicDentistsByClinicFn: func(ctx context.Context, id string) (int64, error) {
			calls = append(calls, "EndClinicDentistsByClinic")
			return 1, nil
		},
		deleteBankAccountsByClinicFn: func(ctx context.Context, id string) (int64, error) {
			calls = append(calls, "DeleteBankAccountsByClinicID")
			return 1, nil
		},
		deleteClinicFn: func(ctx context.Context, id string) (int64, error) {
			calls = append(calls, "DeleteClinic")
			return 1, nil
		},
		deletePersonFn: func(ctx context.Context, id string) (int64, error) {
			calls = append(calls, "DeletePerson")
			return 1, nil
		},
	}

	svc := &Service{}
	if err := svc.deleteClinicWithinTx(context.Background(), q, clinicID); err != nil {
		t.Fatalf("delete clinic within tx: %v", err)
	}

	want := []string{
		"GetClinicByID",
		"LockClinicForUpdate",
		"EndClinicDentistsByClinic",
		"DeleteBankAccountsByClinicID",
		"DeleteClinic",
		"DeletePerson",
	}

	if len(calls) != len(want) {
		t.Fatalf("unexpected number of calls: got %d, want %d (%v)", len(calls), len(want), calls)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Fatalf("unexpected call order at index %d: got %q, want %q (full calls: %v)", i, calls[i], want[i], calls)
		}
	}
}

func TestEnsureUserCreatesWhenMissing(t *testing.T) {
	created := false
	q := mockQuerier{
		getUserByEmailFn: func(ctx context.Context, email string) (repository.User, error) {
			return repository.User{}, sql.ErrNoRows
		},
		createUserFn: func(ctx context.Context, arg repository.CreateUserParams) (repository.User, error) {
			created = true
			if arg.Email != "admin@example.com" {
				t.Fatalf("unexpected email: %s", arg.Email)
			}
			if err := bcrypt.CompareHashAndPassword([]byte(arg.PasswordHash), []byte("secret123")); err != nil {
				t.Fatalf("password hash does not match input password: %v", err)
			}
			return repository.User{ID: arg.ID, Email: arg.Email, PasswordHash: arg.PasswordHash}, nil
		},
	}
	svc := newAuthServiceForTest(q)

	if err := svc.EnsureUser(context.Background(), "admin@example.com", "secret123"); err != nil {
		t.Fatalf("ensure user: %v", err)
	}
	if !created {
		t.Fatalf("expected CreateUser to be called")
	}
}

func TestLoginAndValidateAccessToken(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("generate password hash: %v", err)
	}
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuidv7: %v", err)
	}
	userID := id.String()

	q := mockQuerier{
		getUserByEmailFn: func(ctx context.Context, email string) (repository.User, error) {
			if email != "admin@example.com" {
				return repository.User{}, sql.ErrNoRows
			}
			return repository.User{ID: userID, Email: "admin@example.com", PasswordHash: string(hash)}, nil
		},
	}
	svc := newAuthServiceForTest(q)

	output, err := svc.Login(context.Background(), LoginInput{Email: "admin@example.com", Password: "secret123"})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if output.AccessToken == "" {
		t.Fatalf("expected access token")
	}
	if output.TokenType != "Bearer" {
		t.Fatalf("expected token type Bearer, got %q", output.TokenType)
	}

	if err := svc.ValidateAccessToken(output.AccessToken); err != nil {
		t.Fatalf("validate access token: %v", err)
	}
}

func TestLoginInvalidCredentials(t *testing.T) {
	q := mockQuerier{
		getUserByEmailFn: func(ctx context.Context, email string) (repository.User, error) {
			return repository.User{}, sql.ErrNoRows
		},
	}
	svc := newAuthServiceForTest(q)

	_, err := svc.Login(context.Background(), LoginInput{Email: "wrong@example.com", Password: "invalid-password"})
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got: %v", err)
	}
}
