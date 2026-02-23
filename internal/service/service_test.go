package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"capim-test/internal/db/repository"
)

type mockQuerier struct {
	repository.Querier
	getUserByEmailFn func(ctx context.Context, email string) (repository.User, error)
	createUserFn     func(ctx context.Context, arg repository.CreateUserParams) (repository.User, error)
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
