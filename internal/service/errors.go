package service

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrValidation   = errors.New("validation error")
	ErrConflict     = errors.New("conflict")
	ErrUnauthorized = errors.New("unauthorized")
)

func notFoundError(message string) error {
	return fmt.Errorf("%w: %s", ErrNotFound, message)
}

func validationError(message string) error {
	return fmt.Errorf("%w: %s", ErrValidation, message)
}

func conflictError(message string) error {
	return fmt.Errorf("%w: %s", ErrConflict, message)
}

func unauthorizedError(message string) error {
	return fmt.Errorf("%w: %s", ErrUnauthorized, message)
}
