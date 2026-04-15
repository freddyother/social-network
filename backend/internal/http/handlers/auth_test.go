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
	registerFunc    func(ctx context.Context, input auth.RegisterInput) (auth.AuthResult, error)
	loginFunc       func(ctx context.Context, input auth.LoginInput) (auth.AuthResult, error)
	logoutFunc      func(ctx context.Context, sessionID string) error
	currentUserFunc func(ctx context.Context, sessionID string) (*auth.User, error)
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
