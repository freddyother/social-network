package auth

import "testing"

func TestNormalizeRegisterInputAcceptsISODateOfBirth(t *testing.T) {
	t.Parallel()

	input := RegisterInput{
		Email:       "ada@example.com",
		Password:    "password123",
		FirstName:   "Ada",
		LastName:    "Lovelace",
		DateOfBirth: "1815-12-10",
	}

	normalized, birthDate, err := normalizeRegisterInput(input)
	if err != nil {
		t.Fatalf("normalizeRegisterInput returned error: %v", err)
	}

	if normalized.DateOfBirth != "1815-12-10" {
		t.Fatalf("expected normalized ISO date, got %q", normalized.DateOfBirth)
	}

	if got := birthDate.Format("2006-01-02"); got != "1815-12-10" {
		t.Fatalf("expected parsed birth date 1815-12-10, got %q", got)
	}
}

func TestNormalizeRegisterInputAcceptsSlashDateOfBirth(t *testing.T) {
	t.Parallel()

	input := RegisterInput{
		Email:       "ada@example.com",
		Password:    "password123",
		FirstName:   "Ada",
		LastName:    "Lovelace",
		DateOfBirth: "10/12/1815",
	}

	normalized, birthDate, err := normalizeRegisterInput(input)
	if err != nil {
		t.Fatalf("normalizeRegisterInput returned error: %v", err)
	}

	if normalized.DateOfBirth != "1815-12-10" {
		t.Fatalf("expected normalized ISO date, got %q", normalized.DateOfBirth)
	}

	if got := birthDate.Format("2006-01-02"); got != "1815-12-10" {
		t.Fatalf("expected parsed birth date 1815-12-10, got %q", got)
	}
}

func TestNormalizeRegisterInputRejectsUnsupportedDateOfBirthFormat(t *testing.T) {
	t.Parallel()

	input := RegisterInput{
		Email:       "ada@example.com",
		Password:    "password123",
		FirstName:   "Ada",
		LastName:    "Lovelace",
		DateOfBirth: "1815/12/10",
	}

	_, _, err := normalizeRegisterInput(input)
	if err == nil {
		t.Fatal("expected normalizeRegisterInput to reject unsupported date format")
	}

	validationErr, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if got := validationErr.Fields["dateOfBirth"]; got != "Date of birth must use YYYY-MM-DD or DD/MM/YYYY." {
		t.Fatalf("unexpected dateOfBirth error: %q", got)
	}
}
