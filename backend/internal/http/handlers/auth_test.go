package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"social-network/backend/internal/auth"
	"social-network/backend/internal/config"
)

func TestHandleRegisterDoesNotSetSessionCookie(t *testing.T) {
	t.Parallel()

	handler := NewAuthHandler(stubAuthService{
		registerFunc: func(ctx context.Context, input auth.RegisterInput) (auth.AuthResult, error) {
			return auth.AuthResult{
				User: auth.User{
					ID:                "user-1",
					Email:             input.Email,
					FirstName:         input.FirstName,
					LastName:          input.LastName,
					DateOfBirth:       input.DateOfBirth,
					ProfileVisibility: "public",
					CreatedAt:         time.Now().UTC(),
					UpdatedAt:         time.Now().UTC(),
				},
			}, nil
		},
	}, config.SessionConfig{
		CookieName: "social_network_session",
		TTL:        24 * time.Hour,
	})

	body, err := json.Marshal(map[string]string{
		"email":       "ada@example.com",
		"password":    "password123",
		"firstName":   "Ada",
		"lastName":    "Lovelace",
		"dateOfBirth": "1815-12-10",
		"nickname":    "adal",
	})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleRegister(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 0 {
		t.Fatalf("expected no cookie to be set on register, got %#v", cookies)
	}
}

func TestHandleRegisterReturnsConflictForNicknameAlreadyInUse(t *testing.T) {
	t.Parallel()

	handler := NewAuthHandler(stubAuthService{
		registerFunc: func(ctx context.Context, input auth.RegisterInput) (auth.AuthResult, error) {
			return auth.AuthResult{}, auth.ErrNicknameAlreadyInUse
		},
	}, config.SessionConfig{
		CookieName: "social_network_session",
		TTL:        24 * time.Hour,
	})

	body, err := json.Marshal(map[string]string{
		"email":       "ada@example.com",
		"password":    "password123",
		"firstName":   "Ada",
		"lastName":    "Lovelace",
		"dateOfBirth": "1815-12-10",
		"nickname":    "adal",
	})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleRegister(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}

	var payload struct {
		Error  string            `json:"error"`
		Fields map[string]string `json:"fields"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if payload.Error != "That nickname is already in use." {
		t.Fatalf("unexpected error message: %q", payload.Error)
	}

	if payload.Fields["nickname"] != "That nickname is already in use." {
		t.Fatalf("unexpected nickname field error: %q", payload.Fields["nickname"])
	}
}

func TestHandleNicknameAvailabilityReturnsAvailability(t *testing.T) {
	t.Parallel()

	handler := NewAuthHandler(stubAuthService{
		nicknameAvailabilityFunc: func(ctx context.Context, nickname string) (auth.NicknameAvailability, error) {
			if nickname != "adal" {
				t.Fatalf("expected nickname adal, got %q", nickname)
			}

			return auth.NicknameAvailability{
				Nickname:  nickname,
				Available: false,
			}, nil
		},
	}, config.SessionConfig{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/nickname-availability?nickname=adal", nil)
	rec := httptest.NewRecorder()

	handler.HandleNicknameAvailability(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload auth.NicknameAvailability
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if payload.Nickname != "adal" {
		t.Fatalf("expected nickname adal, got %q", payload.Nickname)
	}

	if payload.Available {
		t.Fatal("expected nickname to be unavailable")
	}
}

func TestHandleForgotPasswordReturnsResetLink(t *testing.T) {
	t.Parallel()

	handler := NewAuthHandler(stubAuthService{
		passwordResetRequestFunc: func(ctx context.Context, input auth.PasswordResetRequestInput) (auth.PasswordResetRequestResult, error) {
			if input.Email != "ada@example.com" {
				t.Fatalf("expected email ada@example.com, got %q", input.Email)
			}

			return auth.PasswordResetRequestResult{
				ResetLink: "http://localhost:5173/reset-password?token=reset-token",
			}, nil
		},
	}, config.SessionConfig{})

	body, err := json.Marshal(map[string]string{
		"email": "ada@example.com",
	})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleForgotPassword(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if payload["message"] != "If the account exists, we sent a password reset link." {
		t.Fatalf("unexpected message: %#v", payload["message"])
	}

	if payload["resetLink"] != "http://localhost:5173/reset-password?token=reset-token" {
		t.Fatalf("unexpected reset link: %#v", payload["resetLink"])
	}
}

func TestHandleResetPasswordReturnsSuccess(t *testing.T) {
	t.Parallel()

	handler := NewAuthHandler(stubAuthService{
		resetPasswordFunc: func(ctx context.Context, input auth.ResetPasswordInput) error {
			if input.Token != "reset-token" {
				t.Fatalf("expected token reset-token, got %q", input.Token)
			}

			if input.NewPassword != "password123" {
				t.Fatalf("expected password123, got %q", input.NewPassword)
			}

			return nil
		},
	}, config.SessionConfig{})

	body, err := json.Marshal(map[string]string{
		"token":       "reset-token",
		"newPassword": "password123",
	})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleResetPassword(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestHandleResetPasswordReturnsValidationErrorForInvalidToken(t *testing.T) {
	t.Parallel()

	handler := NewAuthHandler(stubAuthService{
		resetPasswordFunc: func(ctx context.Context, input auth.ResetPasswordInput) error {
			return auth.ErrInvalidResetToken
		},
	}, config.SessionConfig{})

	body, err := json.Marshal(map[string]string{
		"token":       "bad-token",
		"newPassword": "password123",
	})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleResetPassword(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, rec.Code)
	}
}

func TestHandleLoginSetsSessionCookie(t *testing.T) {
	t.Parallel()

	handler := NewAuthHandler(stubAuthService{
		loginFunc: func(ctx context.Context, input auth.LoginInput) (auth.AuthResult, error) {
			if input.Identifier != "adal" {
				t.Fatalf("expected identifier adal, got %q", input.Identifier)
			}

			if input.Password != "password123" {
				t.Fatalf("expected password123, got %q", input.Password)
			}

			return auth.AuthResult{
				User: auth.User{
					ID:                "user-1",
					Email:             "ada@example.com",
					Nickname:          "adal",
					ProfileVisibility: "public",
					CreatedAt:         time.Now().UTC(),
					UpdatedAt:         time.Now().UTC(),
				},
				Session: auth.Session{
					ID:        "session-1",
					ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
				},
			}, nil
		},
	}, config.SessionConfig{
		CookieName: "social_network_session",
		TTL:        24 * time.Hour,
	})

	body, err := json.Marshal(map[string]string{
		"identifier": "adal",
		"password":   "password123",
	})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleLogin(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie to be set")
	}

	if cookies[0].Name != "social_network_session" || cookies[0].Value != "session-1" {
		t.Fatalf("unexpected cookie: %#v", cookies[0])
	}
}

func TestHandleCurrentUserRequiresAuthentication(t *testing.T) {
	t.Parallel()

	handler := NewAuthHandler(stubAuthService{}, config.SessionConfig{
		CookieName: "social_network_session",
		TTL:        24 * time.Hour,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	rec := httptest.NewRecorder()

	handler.HandleCurrentUser(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestHandleLogoutClearsSessionCookie(t *testing.T) {
	t.Parallel()

	handler := NewAuthHandler(stubAuthService{
		logoutFunc: func(ctx context.Context, sessionID string) error {
			if sessionID != "session-1" {
				t.Fatalf("expected session-1, got %s", sessionID)
			}

			return nil
		},
	}, config.SessionConfig{
		CookieName: "social_network_session",
		TTL:        24 * time.Hour,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "social_network_session", Value: "session-1"})
	rec := httptest.NewRecorder()

	handler.HandleLogout(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) == 0 || cookies[0].MaxAge != -1 {
		t.Fatalf("expected cleared cookie, got %#v", cookies)
	}
}

type stubAuthService struct {
	registerFunc             func(ctx context.Context, input auth.RegisterInput) (auth.AuthResult, error)
	loginFunc                func(ctx context.Context, input auth.LoginInput) (auth.AuthResult, error)
	nicknameAvailabilityFunc func(ctx context.Context, nickname string) (auth.NicknameAvailability, error)
	passwordResetRequestFunc func(ctx context.Context, input auth.PasswordResetRequestInput) (auth.PasswordResetRequestResult, error)
	resetPasswordFunc        func(ctx context.Context, input auth.ResetPasswordInput) error
	logoutFunc               func(ctx context.Context, sessionID string) error
	currentUserFunc          func(ctx context.Context, sessionID string) (*auth.User, error)
}

func (s stubAuthService) Register(ctx context.Context, input auth.RegisterInput) (auth.AuthResult, error) {
	if s.registerFunc == nil {
		return auth.AuthResult{}, nil
	}

	return s.registerFunc(ctx, input)
}

func (s stubAuthService) Login(ctx context.Context, input auth.LoginInput) (auth.AuthResult, error) {
	if s.loginFunc == nil {
		return auth.AuthResult{}, nil
	}

	return s.loginFunc(ctx, input)
}

func (s stubAuthService) CheckNicknameAvailability(ctx context.Context, nickname string) (auth.NicknameAvailability, error) {
	if s.nicknameAvailabilityFunc == nil {
		return auth.NicknameAvailability{}, nil
	}

	return s.nicknameAvailabilityFunc(ctx, nickname)
}

func (s stubAuthService) RequestPasswordReset(ctx context.Context, input auth.PasswordResetRequestInput) (auth.PasswordResetRequestResult, error) {
	if s.passwordResetRequestFunc == nil {
		return auth.PasswordResetRequestResult{}, nil
	}

	return s.passwordResetRequestFunc(ctx, input)
}

func (s stubAuthService) ResetPassword(ctx context.Context, input auth.ResetPasswordInput) error {
	if s.resetPasswordFunc == nil {
		return nil
	}

	return s.resetPasswordFunc(ctx, input)
}

func (s stubAuthService) Logout(ctx context.Context, sessionID string) error {
	if s.logoutFunc == nil {
		return nil
	}

	return s.logoutFunc(ctx, sessionID)
}

func (s stubAuthService) CurrentUser(ctx context.Context, sessionID string) (*auth.User, error) {
	if s.currentUserFunc == nil {
		return nil, auth.ErrUnauthorized
	}

	return s.currentUserFunc(ctx, sessionID)
}
