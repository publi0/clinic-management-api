package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"capim-test/internal/db/repository"
	"capim-test/internal/validation"
)

type accessTokenClaims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

const dummyPasswordHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

func (s *Service) EnsureUser(ctx context.Context, email string, password string) error {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if !validation.ValidateEmail(normalizedEmail) {
		return validationError("invalid email")
	}
	if len(password) < 8 {
		return validationError("password must have at least 8 characters")
	}

	_, err := s.queries.GetUserByEmail(ctx, normalizedEmail)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	userID, err := newUUIDV7()
	if err != nil {
		return err
	}

	_, err = s.queries.CreateUser(ctx, repository.CreateUserParams{
		ID:           userID,
		Email:        normalizedEmail,
		PasswordHash: string(passwordHash),
	})
	if err != nil {
		if isUniqueConstraintError(err) {
			return nil
		}
		return mapDatabaseError(err)
	}

	return nil
}

func (s *Service) Login(ctx context.Context, input LoginInput) (LoginOutput, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if !validation.ValidateEmail(email) {
		return LoginOutput{}, validationError("invalid email")
	}
	if strings.TrimSpace(input.Password) == "" {
		return LoginOutput{}, validationError("password is required")
	}
	if len(s.jwtSigningKey) == 0 {
		return LoginOutput{}, fmt.Errorf("jwt signing key is not configured")
	}

	user, err := s.queries.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Keep timing close to existing-user path to reduce account enumeration via latency.
			_ = bcrypt.CompareHashAndPassword([]byte(dummyPasswordHash), []byte(input.Password))
			return LoginOutput{}, unauthorizedError("invalid credentials")
		}
		return LoginOutput{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		return LoginOutput{}, unauthorizedError("invalid credentials")
	}

	now := s.now().UTC()
	expiresAt := now.Add(s.jwtAccessTokenTTL)
	claims := accessTokenClaims{
		Email: user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.jwtIssuer,
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(s.jwtSigningKey)
	if err != nil {
		return LoginOutput{}, fmt.Errorf("sign access token: %w", err)
	}

	return LoginOutput{
		AccessToken: signedToken,
		TokenType:   "Bearer",
		ExpiresIn:   int64(time.Until(expiresAt).Seconds()),
		UserID:      user.ID,
		Email:       user.Email,
	}, nil
}

func (s *Service) ValidateAccessToken(token string) error {
	if strings.TrimSpace(token) == "" {
		return unauthorizedError("invalid token")
	}
	if len(s.jwtSigningKey) == 0 {
		return fmt.Errorf("jwt signing key is not configured")
	}

	claims := &accessTokenClaims{}
	parsedToken, err := jwt.ParseWithClaims(
		token,
		claims,
		func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, unauthorizedError("invalid token")
			}
			return s.jwtSigningKey, nil
		},
		jwt.WithIssuer(s.jwtIssuer),
	)
	if err != nil || !parsedToken.Valid {
		return unauthorizedError("invalid token")
	}
	if strings.TrimSpace(claims.Subject) == "" {
		return unauthorizedError("invalid token")
	}

	return nil
}
