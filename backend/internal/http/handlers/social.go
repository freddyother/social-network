package handlers

import (
	"context"
	"errors"
	"mime/multipart"
	stdlibhttp "net/http"
	"strings"

	"social-network/backend/internal/auth"
	"social-network/backend/internal/config"
	"social-network/backend/internal/http/response"
	"social-network/backend/internal/social"
)

type socialService interface {
	CreatePost(ctx context.Context, author auth.User, input social.CreatePostInput) (social.Post, error)
	Feed(ctx context.Context, viewerID string) ([]social.Post, error)
	Comments(ctx context.Context, viewerID, postID string) ([]social.Comment, error)
	CreateComment(ctx context.Context, author auth.User, postID string, input social.CreateCommentInput) (social.Comment, error)
	DiscoverUsers(ctx context.Context, viewerID string) ([]social.SuggestedUser, error)
	FollowUser(ctx context.Context, followerID, followeeID string) (social.FollowActionResult, error)
	IncomingFollowRequests(ctx context.Context, userID string) ([]social.FollowRequest, error)
	RespondToFollowRequest(ctx context.Context, userID, requestID string, accept bool) error
	UpdateProfileVisibility(ctx context.Context, userID, visibility string) (auth.User, error)
	UpdateThemePreference(ctx context.Context, userID, themePreference string) (auth.User, error)
	Notifications(ctx context.Context, userID string) ([]social.Notification, error)
	MarkNotificationRead(ctx context.Context, userID, notificationID string) error
}

type SocialHandler struct {
	auth    authService
	service socialService
	session config.SessionConfig
}

func NewSocialHandler(authService authService, service socialService, session config.SessionConfig) SocialHandler {
	return SocialHandler{
		auth:    authService,
		service: service,
		session: session,
	}
}

func (h SocialHandler) HandleFeed(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	posts, err := h.service.Feed(r.Context(), currentUser.ID)
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"posts": posts,
	})
}

func (h SocialHandler) HandleCreatePost(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	if err := r.ParseMultipartForm(64 << 20); err != nil {
		writeError(w, stdlibhttp.StatusBadRequest, "Post form data is invalid.", nil)
		return
	}

	var images []*multipart.FileHeader
	if r.MultipartForm != nil {
		images = r.MultipartForm.File["images"]
	}

	post, err := h.service.CreatePost(r.Context(), *currentUser, social.CreatePostInput{
		Title:   r.FormValue("title"),
		Body:    r.FormValue("body"),
		Privacy: r.FormValue("privacy"),
		Images:  images,
	})
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusCreated, map[string]any{
		"post": post,
	})
}

func (h SocialHandler) HandleComments(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	comments, err := h.service.Comments(r.Context(), currentUser.ID, strings.TrimSpace(r.PathValue("postID")))
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"comments": comments,
	})
}

func (h SocialHandler) HandleCreateComment(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	var payload struct {
		Body            string `json:"body"`
		ParentCommentID string `json:"parentCommentId"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, stdlibhttp.StatusBadRequest, err.Error(), nil)
		return
	}

	comment, err := h.service.CreateComment(
		r.Context(),
		*currentUser,
		strings.TrimSpace(r.PathValue("postID")),
		social.CreateCommentInput{
			Body:            payload.Body,
			ParentCommentID: payload.ParentCommentID,
		},
	)
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusCreated, map[string]any{
		"comment": comment,
	})
}

func (h SocialHandler) HandleDiscoverUsers(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	users, err := h.service.DiscoverUsers(r.Context(), currentUser.ID)
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"users": users,
	})
}

func (h SocialHandler) HandleFollowUser(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	result, err := h.service.FollowUser(r.Context(), currentUser.ID, strings.TrimSpace(r.PathValue("userID")))
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, result)
}

func (h SocialHandler) HandleFollowRequests(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	requests, err := h.service.IncomingFollowRequests(r.Context(), currentUser.ID)
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"requests": requests,
	})
}

func (h SocialHandler) HandleAcceptFollowRequest(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	h.handleFollowRequestResponse(w, r, true)
}

func (h SocialHandler) HandleDeclineFollowRequest(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	h.handleFollowRequestResponse(w, r, false)
}

func (h SocialHandler) HandleUpdateProfileVisibility(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	var payload struct {
		Visibility string `json:"visibility"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, stdlibhttp.StatusBadRequest, err.Error(), nil)
		return
	}

	user, err := h.service.UpdateProfileVisibility(r.Context(), currentUser.ID, payload.Visibility)
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"user": user,
	})
}

func (h SocialHandler) HandleUpdateThemePreference(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	var payload struct {
		ThemePreference string `json:"themePreference"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, stdlibhttp.StatusBadRequest, err.Error(), nil)
		return
	}

	user, err := h.service.UpdateThemePreference(r.Context(), currentUser.ID, payload.ThemePreference)
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"user": user,
	})
}

func (h SocialHandler) HandleNotifications(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	notifications, err := h.service.Notifications(r.Context(), currentUser.ID)
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"notifications": notifications,
	})
}

func (h SocialHandler) HandleMarkNotificationRead(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	if err := h.service.MarkNotificationRead(r.Context(), currentUser.ID, strings.TrimSpace(r.PathValue("notificationID"))); err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"status": "read",
	})
}

func (h SocialHandler) handleFollowRequestResponse(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request, accept bool) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	if err := h.service.RespondToFollowRequest(r.Context(), currentUser.ID, strings.TrimSpace(r.PathValue("requestID")), accept); err != nil {
		h.handleSocialError(w, err)
		return
	}

	status := "declined"
	if accept {
		status = "accepted"
	}

	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"status": status,
	})
}

func (h SocialHandler) requireCurrentUser(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) (*auth.User, bool) {
	user, err := h.auth.CurrentUser(r.Context(), h.sessionIDFromRequest(r))
	if err != nil {
		h.handleSocialError(w, err)
		return nil, false
	}

	return user, true
}

func (h SocialHandler) sessionIDFromRequest(r *stdlibhttp.Request) string {
	cookie, err := r.Cookie(h.session.CookieName)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(cookie.Value)
}

func (h SocialHandler) handleSocialError(w stdlibhttp.ResponseWriter, err error) {
	var authValidationErr *auth.ValidationError
	var socialValidationErr *social.ValidationError

	switch {
	case errors.As(err, &authValidationErr):
		writeError(w, stdlibhttp.StatusUnprocessableEntity, authValidationErr.Message, authValidationErr.Fields)
	case errors.As(err, &socialValidationErr):
		writeError(w, stdlibhttp.StatusUnprocessableEntity, socialValidationErr.Message, socialValidationErr.Fields)
	case errors.Is(err, auth.ErrUnauthorized):
		writeError(w, stdlibhttp.StatusUnauthorized, "Authentication required.", nil)
	case errors.Is(err, social.ErrForbidden):
		writeError(w, stdlibhttp.StatusForbidden, "You are not allowed to do that.", nil)
	case errors.Is(err, social.ErrNotFound):
		writeError(w, stdlibhttp.StatusNotFound, "The requested resource was not found.", nil)
	case errors.Is(err, social.ErrFollowYourself):
		writeError(w, stdlibhttp.StatusUnprocessableEntity, "You cannot follow yourself.", map[string]string{
			"user": "You cannot follow yourself.",
		})
	case errors.Is(err, social.ErrAlreadyHandled):
		writeError(w, stdlibhttp.StatusConflict, "That follow request has already been handled.", nil)
	default:
		writeError(w, stdlibhttp.StatusInternalServerError, "Something went wrong on the server.", nil)
	}
}
