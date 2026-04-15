package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrEmailAlreadyInUse  = errors.New("email already in use")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUnauthorized       = errors.New("authentication required")
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
	Email    string `json:"email"`
	Password string `json:"password"`
}

type Service struct {
	db         *sql.DB
	sessionTTL time.Duration
}

func NewService(db *sql.DB, sessionTTL time.Duration) Service {
	if sessionTTL <= 0 {
		sessionTTL = 30 * 24 * time.Hour
	}

	return Service{
		db:         db,
		sessionTTL: sessionTTL,
	}
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
		if isUniqueViolation(err) {
			return AuthResult{}, ErrEmailAlreadyInUse
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

	record, err := s.findUserByEmail(ctx, normalizedInput.Email)
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
		email,
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

	if len(normalized.Nickname) > 80 {
		fieldErrors["nickname"] = "Nickname must be 80 characters or fewer."
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

func normalizeLoginInput(input LoginInput) (LoginInput, error) {
	normalized := LoginInput{
		Email:    strings.ToLower(strings.TrimSpace(input.Email)),
		Password: input.Password,
	}

	fieldErrors := make(map[string]string)
	if normalized.Email == "" {
		fieldErrors["email"] = "Email is required."
	}

	if normalized.Password == "" {
		fieldErrors["password"] = "Password is required."
	}

	if len(fieldErrors) > 0 {
		return LoginInput{}, &ValidationError{
			Message: "Email and password are required.",
			Fields:  fieldErrors,
		}
	}

	return normalized, nil
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

func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505"
}
