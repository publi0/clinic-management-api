package validation

import (
	"net/mail"
	"regexp"
	"strings"

	"github.com/inovacc/brdoc"
)

var nonDigits = regexp.MustCompile(`\D`)
var nonAlphanumeric = regexp.MustCompile(`[^0-9A-Za-z]`)

func NormalizeCPF(raw string) string {
	return nonDigits.ReplaceAllString(raw, "")
}

func NormalizeCNPJ(raw string) string {
	cleaned := nonAlphanumeric.ReplaceAllString(strings.TrimSpace(raw), "")
	return strings.ToUpper(cleaned)
}

func ValidateEmail(email string) bool {
	email = strings.TrimSpace(email)
	if email == "" {
		return false
	}
	_, err := mail.ParseAddress(email)
	return err == nil
}

func ValidateCPF(cpf string) bool {
	return brdoc.NewCPF().Validate(cpf)
}

func ValidateCNPJ(cnpj string) bool {
	return brdoc.NewCNPJ().Validate(cnpj)
}
