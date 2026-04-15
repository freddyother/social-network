package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrEmailAlreadyInUse    = errors.New("email already in use")
	ErrNicknameAlreadyInUse = errors.New("nickname already in use")
	ErrInvalidCredentials   = errors.New("invalid email or password")
	ErrUnauthorized         = errors.New("authentication required")
	ErrInvalidResetToken    = errors.New("invalid or expired password reset token")
)

type ValidationError struct {
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func (e *ValidationError) Error() string {
	if e == nil || e.Message == "" {
		return "validation failed"
	}

	return e.Message
}

type User struct {
	ID                string    `json:"id"`
	Email             string    `json:"email"`
	FirstName         string    `json:"firstName"`
	LastName          string    `json:"lastName"`
	DateOfBirth       string    `json:"dateOfBirth"`
	AvatarURL         string    `json:"avatarUrl,omitempty"`
	Nickname          string    `json:"nickname,omitempty"`
	AboutMe           string    `json:"aboutMe,omitempty"`
	ProfileVisibility string    `json:"profileVisibility"`
	ThemePreference   string    `json:"themePreference"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type Session struct {
	ID        string
	ExpiresAt time.Time
}

type AuthResult struct {
	User    User
	Session Session
}

type PasswordResetConfig struct {
	TokenTTL             time.Duration
	ResetURL             string
	RevealLinkInResponse bool
}

type NicknameAvailability struct {
	Nickname  string `json:"nickname"`
	Available bool   `json:"available"`
}

type PasswordResetRequestInput struct {
	Email string `json:"email"`
}

type PasswordResetRequestResult struct {
	ResetLink string `json:"resetLink,omitempty"`
}

type ResetPasswordInput struct {
	Token       string `json:"token"`
	NewPassword string `json:"newPassword"`
}

type RegisterInput struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
	DateOfBirth string `json:"dateOfBirth"`
	Nickname    string `json:"nickname"`
	AboutMe     string `json:"aboutMe"`
}

type LoginInput struct {
	Identifier string `json:"identifier"`
	Email      string `json:"email,omitempty"`
	Password   string `json:"password"`
}

type Service struct {
	db            *sql.DB
	sessionTTL    time.Duration
	passwordReset PasswordResetConfig
	resetMailer   PasswordResetMailer
}

func NewService(db *sql.DB, sessionTTL time.Duration, passwordReset PasswordResetConfig, resetMailer PasswordResetMailer) Service {
	if sessionTTL <= 0 {
		sessionTTL = 30 * 24 * time.Hour
	}

	if passwordReset.TokenTTL <= 0 {
		passwordReset.TokenTTL = 30 * time.Minute
	}

	return Service{
		db:            db,
		sessionTTL:    sessionTTL,
		passwordReset: passwordReset,
		resetMailer:   resetMailer,
	}
}

func (s Service) RequestPasswordReset(ctx context.Context, input PasswordResetRequestInput) (PasswordResetRequestResult, error) {
	normalizedInput, err := normalizePasswordResetRequestInput(input)
	if err != nil {
		return PasswordResetRequestResult{}, err
	}

	if s.resetMailer == nil && !s.passwordReset.RevealLinkInResponse {
		return PasswordResetRequestResult{}, fmt.Errorf("password reset delivery is not configured")
	}

	record, err := s.findUserByEmail(ctx, normalizedInput.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PasswordResetRequestResult{}, nil
		}

		return PasswordResetRequestResult{}, err
	}

	resetToken, err := newToken(32)
	if err != nil {
		return PasswordResetRequestResult{}, fmt.Errorf("generate password reset token: %w", err)
	}

	resetLink, err := s.buildPasswordResetLink(resetToken)
	if err != nil {
		return PasswordResetRequestResult{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return PasswordResetRequestResult{}, fmt.Errorf("begin password reset transaction: %w", err)
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(
		ctx,
		`DELETE FROM password_reset_tokens WHERE user_id = $1 OR used_at IS NOT NULL OR expires_at <= $2`,
		record.ID,
		time.Now().UTC(),
	); err != nil {
		return PasswordResetRequestResult{}, fmt.Errorf("clear password reset tokens: %w", err)
	}

	resetTokenID, err := newToken(16)
	if err != nil {
		return PasswordResetRequestResult{}, fmt.Errorf("generate password reset token id: %w", err)
	}

	expiresAt := time.Now().UTC().Add(s.passwordReset.TokenTTL)
	if _, err = tx.ExecContext(
		ctx,
		`
			INSERT INTO password_reset_tokens (
				id,
				user_id,
				token_hash,
				expires_at
			)
			VALUES ($1, $2, $3, $4)
		`,
		resetTokenID,
		record.ID,
		hashResetToken(resetToken),
		expiresAt,
	); err != nil {
		return PasswordResetRequestResult{}, fmt.Errorf("insert password reset token: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return PasswordResetRequestResult{}, fmt.Errorf("commit password reset transaction: %w", err)
	}

	if s.resetMailer != nil {
		if err := s.resetMailer.SendPasswordResetEmail(ctx, record.Email, displayName(record), resetLink); err != nil {
			log.Printf("password reset email send failed for %s: %v", record.Email, err)
			return PasswordResetRequestResult{}, fmt.Errorf("send password reset email: %w", err)
		}

		return PasswordResetRequestResult{}, nil
	}

	if s.passwordReset.RevealLinkInResponse {
		log.Printf("password reset link for %s: %s", record.Email, resetLink)
		return PasswordResetRequestResult{ResetLink: resetLink}, nil
	}

	return PasswordResetRequestResult{}, nil
}

func (s Service) ResetPassword(ctx context.Context, input ResetPasswordInput) error {
	normalizedInput, err := normalizeResetPasswordInput(input)
	if err != nil {
		return err
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(normalizedInput.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin password reset transaction: %w", err)
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var resetTokenID string
	var userID string
	if err = tx.QueryRowContext(
		ctx,
		`
			SELECT id, user_id
			FROM password_reset_tokens
			WHERE token_hash = $1 AND used_at IS NULL AND expires_at > $2
		`,
		hashResetToken(normalizedInput.Token),
		time.Now().UTC(),
	).Scan(&resetTokenID, &userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrInvalidResetToken
		}

		return fmt.Errorf("load password reset token: %w", err)
	}

	if _, err = tx.ExecContext(
		ctx,
		`UPDATE users SET password_hash = $1, updated_at = $2 WHERE id = $3`,
		string(passwordHash),
		time.Now().UTC(),
		userID,
	); err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	if _, err = tx.ExecContext(
		ctx,
		`UPDATE password_reset_tokens SET used_at = $1 WHERE id = $2`,
		time.Now().UTC(),
		resetTokenID,
	); err != nil {
		return fmt.Errorf("mark password reset token used: %w", err)
	}

	if _, err = tx.ExecContext(ctx, `DELETE FROM password_reset_tokens WHERE user_id = $1 AND id <> $2`, userID, resetTokenID); err != nil {
		return fmt.Errorf("clear other password reset tokens: %w", err)
	}

	if _, err = tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("clear sessions after password reset: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit password reset transaction: %w", err)
	}

	return nil
}

func (s Service) CheckNicknameAvailability(ctx context.Context, nickname string) (NicknameAvailability, error) {
	normalizedNickname := strings.TrimSpace(nickname)
	if nicknameError := validateNickname(normalizedNickname); nicknameError != "" {
		return NicknameAvailability{}, &ValidationError{
			Message: "Please correct the highlighted fields.",
			Fields: map[string]string{
				"nickname": nicknameError,
			},
		}
	}

	exists, err := s.nicknameExists(ctx, normalizedNickname)
	if err != nil {
		return NicknameAvailability{}, err
	}

	return NicknameAvailability{
		Nickname:  normalizedNickname,
		Available: !exists,
	}, nil
}

func (s Service) Register(ctx context.Context, input RegisterInput) (AuthResult, error) {
	normalizedInput, birthDate, err := normalizeRegisterInput(input)
	if err != nil {
		return AuthResult{}, err
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(normalizedInput.Password), bcrypt.DefaultCost)
	if err != nil {
		return AuthResult{}, fmt.Errorf("hash password: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AuthResult{}, fmt.Errorf("begin register transaction: %w", err)
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	userID, err := newToken(16)
	if err != nil {
		return AuthResult{}, fmt.Errorf("generate user id: %w", err)
	}

	row := tx.QueryRowContext(
		ctx,
		`
			INSERT INTO users (
				id,
				email,
				password_hash,
				first_name,
				last_name,
				date_of_birth,
				nickname,
				about_me,
				profile_visibility
			)
			VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), NULLIF($8, ''), 'public')
			RETURNING
				id,
				email,
				password_hash,
				first_name,
				last_name,
				date_of_birth,
				avatar_url,
				nickname,
				about_me,
				profile_visibility,
				theme_preference,
				created_at,
				updated_at
		`,
		userID,
		normalizedInput.Email,
		string(passwordHash),
		normalizedInput.FirstName,
		normalizedInput.LastName,
		birthDate,
		normalizedInput.Nickname,
		normalizedInput.AboutMe,
	)

	record, err := scanUserRecord(row)
	if err != nil {
		switch uniqueViolationConstraint(err) {
		case "users_email_key":
			return AuthResult{}, ErrEmailAlreadyInUse
		case "idx_users_nickname_unique":
			return AuthResult{}, ErrNicknameAlreadyInUse
		}

		return AuthResult{}, fmt.Errorf("insert user: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return AuthResult{}, fmt.Errorf("commit register transaction: %w", err)
	}

	return AuthResult{
		User: record.PublicUser(),
	}, nil
}

func (s Service) Login(ctx context.Context, input LoginInput) (AuthResult, error) {
	normalizedInput, err := normalizeLoginInput(input)
	if err != nil {
		return AuthResult{}, err
	}

	record, err := s.findUserByLoginIdentifier(ctx, normalizedInput.Identifier)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AuthResult{}, ErrInvalidCredentials
		}

		return AuthResult{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(record.PasswordHash), []byte(normalizedInput.Password)); err != nil {
		return AuthResult{}, ErrInvalidCredentials
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AuthResult{}, fmt.Errorf("begin login transaction: %w", err)
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	session, err := s.createSession(ctx, tx, record.ID)
	if err != nil {
		return AuthResult{}, err
	}

	if err = tx.Commit(); err != nil {
		return AuthResult{}, fmt.Errorf("commit login transaction: %w", err)
	}

	return AuthResult{
		User:    record.PublicUser(),
		Session: session,
	}, nil
}

func (s Service) Logout(ctx context.Context, sessionID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return nil
	}

	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = $1`, sessionID); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	return nil
}

func (s Service) CurrentUser(ctx context.Context, sessionID string) (*User, error) {
	trimmedSessionID := strings.TrimSpace(sessionID)
	if trimmedSessionID == "" {
		return nil, ErrUnauthorized
	}

	row := s.db.QueryRowContext(
		ctx,
		`
			SELECT
				u.id,
				u.email,
				u.password_hash,
				u.first_name,
				u.last_name,
				u.date_of_birth,
				u.avatar_url,
				u.nickname,
				u.about_me,
				u.profile_visibility,
				u.theme_preference,
				u.created_at,
				u.updated_at
			FROM sessions s
			INNER JOIN users u ON u.id = s.user_id
			WHERE s.id = $1 AND s.expires_at > $2
		`,
		trimmedSessionID,
		time.Now().UTC(),
	)

	record, err := scanUserRecord(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUnauthorized
		}

		return nil, fmt.Errorf("load current user: %w", err)
	}

	user := record.PublicUser()
	return &user, nil
}

func (s Service) findUserByEmail(ctx context.Context, email string) (userRecord, error) {
	row := s.db.QueryRowContext(
		ctx,
		`
			SELECT
				id,
				email,
				password_hash,
				first_name,
				last_name,
				date_of_birth,
				avatar_url,
				nickname,
				about_me,
				profile_visibility,
				theme_preference,
				created_at,
				updated_at
			FROM users
			WHERE email = $1
		`,
		strings.ToLower(strings.TrimSpace(email)),
	)

	record, err := scanUserRecord(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return userRecord{}, sql.ErrNoRows
		}

		return userRecord{}, fmt.Errorf("find user by email: %w", err)
	}

	return record, nil
}

func (s Service) nicknameExists(ctx context.Context, nickname string) (bool, error) {
	var exists bool
	if err := s.db.QueryRowContext(
		ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE LOWER(BTRIM(nickname)) = $1)`,
		strings.ToLower(strings.TrimSpace(nickname)),
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("check nickname existence: %w", err)
	}

	return exists, nil
}

func (s Service) findUserByLoginIdentifier(ctx context.Context, identifier string) (userRecord, error) {
	row := s.db.QueryRowContext(
		ctx,
		`
			SELECT
				id,
				email,
				password_hash,
				first_name,
				last_name,
				date_of_birth,
				avatar_url,
				nickname,
				about_me,
				profile_visibility,
				theme_preference,
				created_at,
				updated_at
			FROM users
			WHERE email = $1 OR LOWER(BTRIM(nickname)) = $1
		`,
		identifier,
	)

	record, err := scanUserRecord(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return userRecord{}, sql.ErrNoRows
		}

		return userRecord{}, fmt.Errorf("find user by login identifier: %w", err)
	}

	return record, nil
}

func (s Service) createSession(ctx context.Context, tx *sql.Tx, userID string) (Session, error) {
	sessionID, err := newToken(32)
	if err != nil {
		return Session{}, fmt.Errorf("generate session id: %w", err)
	}

	expiresAt := time.Now().UTC().Add(s.sessionTTL)
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO sessions (id, user_id, expires_at) VALUES ($1, $2, $3)`,
		sessionID,
		userID,
		expiresAt,
	); err != nil {
		return Session{}, fmt.Errorf("insert session: %w", err)
	}

	return Session{
		ID:        sessionID,
		ExpiresAt: expiresAt,
	}, nil
}

func normalizeRegisterInput(input RegisterInput) (RegisterInput, time.Time, error) {
	normalized := RegisterInput{
		Email:       strings.ToLower(strings.TrimSpace(input.Email)),
		Password:    input.Password,
		FirstName:   strings.TrimSpace(input.FirstName),
		LastName:    strings.TrimSpace(input.LastName),
		DateOfBirth: strings.TrimSpace(input.DateOfBirth),
		Nickname:    strings.TrimSpace(input.Nickname),
		AboutMe:     strings.TrimSpace(input.AboutMe),
	}

	fieldErrors := make(map[string]string)

	if normalized.FirstName == "" {
		fieldErrors["firstName"] = "First name is required."
	}

	if normalized.LastName == "" {
		fieldErrors["lastName"] = "Last name is required."
	}

	if normalized.Email == "" {
		fieldErrors["email"] = "Email is required."
	} else if _, err := mail.ParseAddress(normalized.Email); err != nil {
		fieldErrors["email"] = "Enter a valid email address."
	}

	if len(normalized.Password) < 8 {
		fieldErrors["password"] = "Password must be at least 8 characters."
	}

	birthDate, err := parseDateOfBirth(normalized.DateOfBirth)
	if normalized.DateOfBirth == "" {
		fieldErrors["dateOfBirth"] = "Date of birth is required."
	} else if err != nil {
		fieldErrors["dateOfBirth"] = "Date of birth must use YYYY-MM-DD or DD/MM/YYYY."
	} else if !birthDate.Before(time.Now().UTC()) {
		fieldErrors["dateOfBirth"] = "Date of birth must be in the past."
	} else {
		normalized.DateOfBirth = birthDate.Format("2006-01-02")
	}

	if nicknameError := validateNickname(normalized.Nickname); nicknameError != "" {
		fieldErrors["nickname"] = nicknameError
	}

	if len(normalized.AboutMe) > 500 {
		fieldErrors["aboutMe"] = "About me must be 500 characters or fewer."
	}

	if len(fieldErrors) > 0 {
		return RegisterInput{}, time.Time{}, &ValidationError{
			Message: "Please correct the highlighted fields.",
			Fields:  fieldErrors,
		}
	}

	return normalized, birthDate.UTC(), nil
}

func parseDateOfBirth(value string) (time.Time, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return time.Time{}, fmt.Errorf("date of birth is empty")
	}

	for _, layout := range []string{"2006-01-02", "2/1/2006"} {
		birthDate, err := time.Parse(layout, normalized)
		if err == nil {
			return birthDate.UTC(), nil
		}
	}

	return time.Time{}, fmt.Errorf("date of birth must use YYYY-MM-DD or DD/MM/YYYY")
}

func validateNickname(nickname string) string {
	normalizedNickname := strings.TrimSpace(nickname)

	if normalizedNickname == "" {
		return "Nickname is required."
	}

	if len(normalizedNickname) > 80 {
		return "Nickname must be 80 characters or fewer."
	}

	return ""
}

func normalizePasswordResetRequestInput(input PasswordResetRequestInput) (PasswordResetRequestInput, error) {
	normalized := PasswordResetRequestInput{
		Email: strings.ToLower(strings.TrimSpace(input.Email)),
	}

	fieldErrors := make(map[string]string)
	if normalized.Email == "" {
		fieldErrors["email"] = "Email is required."
	} else if _, err := mail.ParseAddress(normalized.Email); err != nil {
		fieldErrors["email"] = "Enter a valid email address."
	}

	if len(fieldErrors) > 0 {
		return PasswordResetRequestInput{}, &ValidationError{
			Message: "Please correct the highlighted fields.",
			Fields:  fieldErrors,
		}
	}

	return normalized, nil
}

func normalizeResetPasswordInput(input ResetPasswordInput) (ResetPasswordInput, error) {
	normalized := ResetPasswordInput{
		Token:       strings.TrimSpace(input.Token),
		NewPassword: input.NewPassword,
	}

	fieldErrors := make(map[string]string)
	if normalized.Token == "" {
		fieldErrors["token"] = "Reset token is required."
	}

	if len(normalized.NewPassword) < 8 {
		fieldErrors["newPassword"] = "Password must be at least 8 characters."
	}

	if len(fieldErrors) > 0 {
		return ResetPasswordInput{}, &ValidationError{
			Message: "Please correct the highlighted fields.",
			Fields:  fieldErrors,
		}
	}

	return normalized, nil
}

func normalizeLoginInput(input LoginInput) (LoginInput, error) {
	rawIdentifier := input.Identifier
	if strings.TrimSpace(rawIdentifier) == "" {
		rawIdentifier = input.Email
	}

	normalized := LoginInput{
		Identifier: strings.ToLower(strings.TrimSpace(rawIdentifier)),
		Password:   input.Password,
	}

	fieldErrors := make(map[string]string)
	if normalized.Identifier == "" {
		fieldErrors["identifier"] = "Nickname or email is required."
	}

	if normalized.Password == "" {
		fieldErrors["password"] = "Password is required."
	}

	if len(fieldErrors) > 0 {
		return LoginInput{}, &ValidationError{
			Message: "Nickname or email and password are required.",
			Fields:  fieldErrors,
		}
	}

	return normalized, nil
}

func (s Service) buildPasswordResetLink(token string) (string, error) {
	resetURL := strings.TrimSpace(s.passwordReset.ResetURL)
	if resetURL == "" {
		return "", fmt.Errorf("password reset URL is not configured")
	}

	parsedURL, err := url.Parse(resetURL)
	if err != nil {
		return "", fmt.Errorf("parse password reset URL: %w", err)
	}

	query := parsedURL.Query()
	query.Set("token", token)
	parsedURL.RawQuery = query.Encode()

	return parsedURL.String(), nil
}

type scanner interface {
	Scan(dest ...any) error
}

type userRecord struct {
	ID                string
	Email             string
	PasswordHash      string
	FirstName         string
	LastName          string
	DateOfBirth       time.Time
	AvatarURL         sql.NullString
	Nickname          sql.NullString
	AboutMe           sql.NullString
	ProfileVisibility string
	ThemePreference   string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (r userRecord) PublicUser() User {
	return User{
		ID:                r.ID,
		Email:             r.Email,
		FirstName:         r.FirstName,
		LastName:          r.LastName,
		DateOfBirth:       r.DateOfBirth.Format("2006-01-02"),
		AvatarURL:         nullableStringValue(r.AvatarURL),
		Nickname:          nullableStringValue(r.Nickname),
		AboutMe:           nullableStringValue(r.AboutMe),
		ProfileVisibility: r.ProfileVisibility,
		ThemePreference:   r.ThemePreference,
		CreatedAt:         r.CreatedAt,
		UpdatedAt:         r.UpdatedAt,
	}
}

func scanUserRecord(row scanner) (userRecord, error) {
	var record userRecord
	err := row.Scan(
		&record.ID,
		&record.Email,
		&record.PasswordHash,
		&record.FirstName,
		&record.LastName,
		&record.DateOfBirth,
		&record.AvatarURL,
		&record.Nickname,
		&record.AboutMe,
		&record.ProfileVisibility,
		&record.ThemePreference,
		&record.CreatedAt,
		&record.UpdatedAt,
	)
	if err != nil {
		return userRecord{}, err
	}

	return record, nil
}

func nullableStringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}

	return value.String
}

func newToken(size int) (string, error) {
	bytes := make([]byte, size)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return hex.EncodeToString(bytes), nil
}

func hashResetToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func displayName(record userRecord) string {
	if nickname := strings.TrimSpace(nullableStringValue(record.Nickname)); nickname != "" {
		return nickname
	}

	fullName := strings.TrimSpace(strings.Join([]string{record.FirstName, record.LastName}, " "))
	if fullName != "" {
		return fullName
	}

	return record.Email
}

func uniqueViolationConstraint(err error) string {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) || pqErr.Code != "23505" {
		return ""
	}

	return pqErr.Constraint
}
