package validation

import "testing"

func TestNormalizeCNPJ(t *testing.T) {
	got := NormalizeCNPJ("12.abc.345/01de-35")
	if got != "12ABC34501DE35" {
		t.Fatalf("expected 12ABC34501DE35, got %q", got)
	}
}

func TestValidateCNPJAcceptsNumericAndAlphanumeric(t *testing.T) {
	tests := []string{
		"04.252.011/0001-10",
		"12.abc.345/01de-35",
	}

	for _, tc := range tests {
		if !ValidateCNPJ(tc) {
			t.Fatalf("expected valid CNPJ for %q", tc)
		}
	}
}

func TestValidateCNPJRejectsInvalidCheckDigits(t *testing.T) {
	if ValidateCNPJ("12ABC34501DE36") {
		t.Fatalf("expected invalid CNPJ with wrong check digits")
	}
}
