package auth

import (
	"testing"

	"github.com/lib/pq"
)

func TestNormalizeRegisterInputAcceptsISODateOfBirth(t *testing.T) {
	t.Parallel()

	input := RegisterInput{
		Email:       "ada@example.com",
		Password:    "password123",
		FirstName:   "Ada",
		LastName:    "Lovelace",
		DateOfBirth: "1815-12-10",
		Nickname:    "adal",
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
		Nickname:    "adal",
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
		Nickname:    "adal",
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

func TestNormalizeRegisterInputRequiresNickname(t *testing.T) {
	t.Parallel()

	input := RegisterInput{
		Email:       "ada@example.com",
		Password:    "password123",
		FirstName:   "Ada",
		LastName:    "Lovelace",
		DateOfBirth: "1815-12-10",
		Nickname:    "",
	}

	_, _, err := normalizeRegisterInput(input)
	if err == nil {
		t.Fatal("expected normalizeRegisterInput to require nickname")
	}

	validationErr, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if got := validationErr.Fields["nickname"]; got != "Nickname is required." {
		t.Fatalf("unexpected nickname error: %q", got)
	}
}

func TestValidateNicknameRejectsTooLongNickname(t *testing.T) {
	t.Parallel()

	nickname := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	if got := validateNickname(nickname); got != "Nickname must be 80 characters or fewer." {
		t.Fatalf("unexpected nickname validation error: %q", got)
	}
}

func TestUniqueViolationConstraintReturnsConstraintName(t *testing.T) {
	t.Parallel()

	err := &pq.Error{
		Code:       "23505",
		Constraint: "idx_users_nickname_unique",
	}

	if got := uniqueViolationConstraint(err); got != "idx_users_nickname_unique" {
		t.Fatalf("expected constraint name, got %q", got)
	}
}

func TestUniqueViolationConstraintReturnsEmptyForNonUniqueErrors(t *testing.T) {
	t.Parallel()

	err := &pq.Error{
		Code:       "23503",
		Constraint: "users_email_key",
	}

	if got := uniqueViolationConstraint(err); got != "" {
		t.Fatalf("expected empty constraint name, got %q", got)
	}
}

func TestNormalizeLoginInputAcceptsIdentifier(t *testing.T) {
	t.Parallel()

	normalized, err := normalizeLoginInput(LoginInput{
		Identifier: " Win ",
		Password:   "password123",
	})
	if err != nil {
		t.Fatalf("normalizeLoginInput returned error: %v", err)
	}

	if normalized.Identifier != "win" {
		t.Fatalf("expected normalized identifier %q, got %q", "win", normalized.Identifier)
	}
}

func TestNormalizeLoginInputFallsBackToEmailField(t *testing.T) {
	t.Parallel()

	normalized, err := normalizeLoginInput(LoginInput{
		Email:    " Ada@example.com ",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("normalizeLoginInput returned error: %v", err)
	}

	if normalized.Identifier != "ada@example.com" {
		t.Fatalf("expected normalized identifier %q, got %q", "ada@example.com", normalized.Identifier)
	}
}

func TestNormalizePasswordResetRequestInputRequiresEmail(t *testing.T) {
	t.Parallel()

	_, err := normalizePasswordResetRequestInput(PasswordResetRequestInput{})
	if err == nil {
		t.Fatal("expected normalizePasswordResetRequestInput to return an error")
	}

	validationErr, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if got := validationErr.Fields["email"]; got != "Email is required." {
		t.Fatalf("unexpected email error: %q", got)
	}
}

func TestNormalizeResetPasswordInputRequiresTokenAndPassword(t *testing.T) {
	t.Parallel()

	_, err := normalizeResetPasswordInput(ResetPasswordInput{})
	if err == nil {
		t.Fatal("expected normalizeResetPasswordInput to return an error")
	}

	validationErr, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if got := validationErr.Fields["token"]; got != "Reset token is required." {
		t.Fatalf("unexpected token error: %q", got)
	}

	if got := validationErr.Fields["newPassword"]; got != "Password must be at least 8 characters." {
		t.Fatalf("unexpected newPassword error: %q", got)
	}
}

func TestBuildPasswordResetLinkAddsTokenQuery(t *testing.T) {
	t.Parallel()

	service := Service{
		passwordReset: PasswordResetConfig{
			ResetURL: "http://localhost:5173/reset-password",
		},
	}

	link, err := service.buildPasswordResetLink("reset-token")
	if err != nil {
		t.Fatalf("buildPasswordResetLink returned error: %v", err)
	}

	if link != "http://localhost:5173/reset-password?token=reset-token" {
		t.Fatalf("unexpected reset link: %q", link)
	}
}
