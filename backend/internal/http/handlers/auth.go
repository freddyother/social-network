package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	stdlibhttp "net/http"
	"strings"
	"time"

	"social-network/backend/internal/auth"
	"social-network/backend/internal/config"
	"social-network/backend/internal/http/response"
)

type authService interface {
	Register(ctx context.Context, input auth.RegisterInput) (auth.AuthResult, error)
	Login(ctx context.Context, input auth.LoginInput) (auth.AuthResult, error)
	OAuthLogin(ctx context.Context, input auth.OAuthLoginInput) (auth.AuthResult, error)
	CheckNicknameAvailability(ctx context.Context, nickname string) (auth.NicknameAvailability, error)
	RequestPasswordReset(ctx context.Context, input auth.PasswordResetRequestInput) (auth.PasswordResetRequestResult, error)
	ResetPassword(ctx context.Context, input auth.ResetPasswordInput) error
	Logout(ctx context.Context, sessionID string) error
	CurrentUser(ctx context.Context, sessionID string) (*auth.User, error)
}

type AuthHandler struct {
	service authService
	session config.SessionConfig
}

func NewAuthHandler(service authService, session config.SessionConfig) AuthHandler {
	return AuthHandler{
		service: service,
		session: session,
	}
}

func (h AuthHandler) HandleRegister(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	var input auth.RegisterInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, stdlibhttp.StatusBadRequest, err.Error(), nil)
		return
	}

	result, err := h.service.Register(r.Context(), input)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusCreated, map[string]any{
		"user": result.User,
	})
}

func (h AuthHandler) HandleLogin(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	var input auth.LoginInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, stdlibhttp.StatusBadRequest, err.Error(), nil)
		return
	}

	result, err := h.service.Login(r.Context(), input)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.setSessionCookie(w, result.Session)
	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"user": result.User,
	})
}

func (h AuthHandler) HandleOAuthLogin(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	var input auth.OAuthLoginInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, stdlibhttp.StatusBadRequest, err.Error(), nil)
		return
	}

	result, err := h.service.OAuthLogin(r.Context(), input)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.setSessionCookie(w, result.Session)
	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"user": result.User,
	})
}

func (h AuthHandler) HandleNicknameAvailability(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	availability, err := h.service.CheckNicknameAvailability(r.Context(), r.URL.Query().Get("nickname"))
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, availability)
}

func (h AuthHandler) HandleForgotPassword(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	var input auth.PasswordResetRequestInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, stdlibhttp.StatusBadRequest, err.Error(), nil)
		return
	}

	result, err := h.service.RequestPasswordReset(r.Context(), input)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	payload := map[string]any{
		"message": "If the account exists, we sent a password reset link.",
	}
	if result.ResetLink != "" {
		payload["resetLink"] = result.ResetLink
	}

	response.JSON(w, stdlibhttp.StatusOK, payload)
}

func (h AuthHandler) HandleResetPassword(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	var input auth.ResetPasswordInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, stdlibhttp.StatusBadRequest, err.Error(), nil)
		return
	}

	if err := h.service.ResetPassword(r.Context(), input); err != nil {
		h.handleServiceError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"message": "Your password has been updated. You can sign in now.",
	})
}

func (h AuthHandler) HandleLogout(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	sessionID := h.sessionIDFromRequest(r)
	if err := h.service.Logout(r.Context(), sessionID); err != nil {
		writeError(w, stdlibhttp.StatusInternalServerError, "Could not log out right now.", nil)
		return
	}

	h.clearSessionCookie(w)
	w.WriteHeader(stdlibhttp.StatusNoContent)
}

func (h AuthHandler) HandleCurrentUser(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	sessionID := h.sessionIDFromRequest(r)
	user, err := h.service.CurrentUser(r.Context(), sessionID)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"user": user,
	})
}

func (h AuthHandler) handleServiceError(w stdlibhttp.ResponseWriter, err error) {
	var validationErr *auth.ValidationError
	switch {
	case errors.As(err, &validationErr):
		writeError(w, stdlibhttp.StatusUnprocessableEntity, validationErr.Message, validationErr.Fields)
	case errors.Is(err, auth.ErrEmailAlreadyInUse):
		writeError(w, stdlibhttp.StatusConflict, "An account with that email already exists.", map[string]string{
			"email": "That email is already in use.",
		})
	case errors.Is(err, auth.ErrNicknameAlreadyInUse):
		writeError(w, stdlibhttp.StatusConflict, "That nickname is already in use.", map[string]string{
			"nickname": "That nickname is already in use.",
		})
	case errors.Is(err, auth.ErrInvalidResetToken):
		writeError(w, stdlibhttp.StatusUnprocessableEntity, "That password reset link is invalid or has expired.", map[string]string{
			"token": "That password reset link is invalid or has expired.",
		})
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(w, stdlibhttp.StatusUnauthorized, "Invalid nickname, email, or password.", nil)
	case errors.Is(err, auth.ErrOAuthNotConfigured):
		writeError(w, stdlibhttp.StatusServiceUnavailable, "Social sign-in is not configured yet.", nil)
	case errors.Is(err, auth.ErrInvalidOAuthToken):
		writeError(w, stdlibhttp.StatusUnauthorized, "Social sign-in could not be verified.", nil)
	case errors.Is(err, auth.ErrUnauthorized):
		writeError(w, stdlibhttp.StatusUnauthorized, "Authentication required.", nil)
	default:
		log.Printf("auth handler error: %T: %v", err, err)
		writeError(w, stdlibhttp.StatusInternalServerError, "Something went wrong on the server.", nil)
	}
}

func (h AuthHandler) setSessionCookie(w stdlibhttp.ResponseWriter, session auth.Session) {
	stdlibhttp.SetCookie(w, &stdlibhttp.Cookie{
		Name:     h.session.CookieName,
		Value:    session.ID,
		Path:     "/",
		Expires:  session.ExpiresAt,
		HttpOnly: true,
		SameSite: stdlibhttp.SameSiteLaxMode,
		Secure:   h.session.Secure,
		MaxAge:   int(time.Until(session.ExpiresAt).Seconds()),
	})
}

func (h AuthHandler) clearSessionCookie(w stdlibhttp.ResponseWriter) {
	stdlibhttp.SetCookie(w, &stdlibhttp.Cookie{
		Name:     h.session.CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: stdlibhttp.SameSiteLaxMode,
		Secure:   h.session.Secure,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0).UTC(),
	})
}

func (h AuthHandler) sessionIDFromRequest(r *stdlibhttp.Request) string {
	cookie, err := r.Cookie(h.session.CookieName)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(cookie.Value)
}

func decodeJSON(r *stdlibhttp.Request, target any) error {
	defer r.Body.Close()

	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		return errors.New("Request body must be valid JSON.")
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("Request body must contain a single JSON object.")
	}

	return nil
}

func writeError(w stdlibhttp.ResponseWriter, status int, message string, fields map[string]string) {
	payload := map[string]any{
		"error": message,
	}
	if len(fields) > 0 {
		payload["fields"] = fields
	}

	response.JSON(w, status, payload)
}
