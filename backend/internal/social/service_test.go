package social

import "testing"

func TestNormalizeUpdateProfileInputAcceptsEditableDateOfBirth(t *testing.T) {
	t.Parallel()

	normalized, err := normalizeUpdateProfileInput(UpdateProfileInput{
		FirstName:   " Ada ",
		LastName:    " Lovelace ",
		DateOfBirth: "10/12/1815",
		AboutMe:     " Mathematics ",
	})
	if err != nil {
		t.Fatalf("normalizeUpdateProfileInput returned error: %v", err)
	}

	if normalized.DateOfBirth != "1815-12-10" {
		t.Fatalf("expected normalized date of birth, got %q", normalized.DateOfBirth)
	}
}

func TestNormalizeUpdateProfileInputAllowsBlankDateOfBirth(t *testing.T) {
	t.Parallel()

	normalized, err := normalizeUpdateProfileInput(UpdateProfileInput{
		FirstName: "Ada",
		LastName:  "Lovelace",
	})
	if err != nil {
		t.Fatalf("normalizeUpdateProfileInput returned error: %v", err)
	}

	if normalized.DateOfBirth != "" {
		t.Fatalf("expected blank date of birth, got %q", normalized.DateOfBirth)
	}
}

func TestNormalizeUpdateProfileInputRejectsFutureDateOfBirth(t *testing.T) {
	t.Parallel()

	_, err := normalizeUpdateProfileInput(UpdateProfileInput{
		FirstName:   "Ada",
		LastName:    "Lovelace",
		DateOfBirth: "2999-01-01",
	})
	if err == nil {
		t.Fatal("expected normalizeUpdateProfileInput to reject a future date")
	}

	validationErr, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if got := validationErr.Fields["dateOfBirth"]; got != "Date of birth must be in the past." {
		t.Fatalf("unexpected dateOfBirth error: %q", got)
	}
}
