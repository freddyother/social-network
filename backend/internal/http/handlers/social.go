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
	UpdatePost(ctx context.Context, author auth.User, postID string, input social.UpdatePostInput) (social.Post, error)
	DeletePost(ctx context.Context, author auth.User, postID string) error
	Feed(ctx context.Context, viewerID string) ([]social.Post, error)
	MyPosts(ctx context.Context, userID string) ([]social.Post, error)
	PostByID(ctx context.Context, viewerID, postID string) (social.Post, error)
	ProfileByHandle(ctx context.Context, viewerID, handle string) (social.PublicProfilePage, error)
	Groups(ctx context.Context, viewerID string) ([]social.Group, error)
	CreateGroup(ctx context.Context, creator auth.User, input social.CreateGroupInput) (social.Group, error)
	GroupByID(ctx context.Context, viewerID, groupID string) (social.Group, error)
	JoinGroup(ctx context.Context, userID, groupID string) (social.Group, error)
	GroupPosts(ctx context.Context, viewerID, groupID string) ([]social.GroupPost, error)
	CreateGroupPost(ctx context.Context, author auth.User, groupID string, input social.CreateGroupPostInput) (social.GroupPost, error)
	GroupComments(ctx context.Context, viewerID, groupID, groupPostID string) ([]social.GroupComment, error)
	CreateGroupComment(ctx context.Context, author auth.User, groupID, groupPostID string, input social.CreateGroupCommentInput) (social.GroupComment, error)
	GroupEvents(ctx context.Context, viewerID, groupID string) ([]social.GroupEvent, error)
	CreateGroupEvent(ctx context.Context, creator auth.User, groupID string, input social.CreateGroupEventInput) (social.GroupEvent, error)
	RespondToGroupEvent(ctx context.Context, userID, groupID, eventID, response string) (social.GroupEvent, error)
	InviteUserToGroup(ctx context.Context, sender auth.User, groupID string, input social.InviteUserToGroupInput) (social.PrivateMessage, error)
	Search(ctx context.Context, viewerID, query string) (social.GlobalSearchResult, error)
	Conversations(ctx context.Context, userID string) ([]social.ConversationSummary, error)
	Conversation(ctx context.Context, viewerID, partnerID string) (social.ConversationThread, error)
	SendPrivateMessage(ctx context.Context, sender auth.User, partnerID string, input social.SendPrivateMessageInput) (social.PrivateMessage, error)
	MarkConversationRead(ctx context.Context, viewerID, partnerID string) (social.ConversationReadResult, error)
	Comments(ctx context.Context, viewerID, postID string) ([]social.Comment, error)
	CreateComment(ctx context.Context, author auth.User, postID string, input social.CreateCommentInput) (social.Comment, error)
	UpdateComment(ctx context.Context, author auth.User, postID, commentID string, input social.UpdateCommentInput) (social.Comment, error)
	DiscoverUsers(ctx context.Context, viewerID string) ([]social.SuggestedUser, error)
	FollowUser(ctx context.Context, followerID, followeeID string) (social.FollowActionResult, error)
	UnfollowUser(ctx context.Context, followerID, followeeID string) (social.FollowActionResult, error)
	IncomingFollowRequests(ctx context.Context, userID string) ([]social.FollowRequest, error)
	RespondToFollowRequest(ctx context.Context, userID, requestID string, accept bool) error
	UpdateProfile(ctx context.Context, userID string, input social.UpdateProfileInput) (auth.User, error)
	UpdateProfileVisibility(ctx context.Context, userID, visibility string) (auth.User, error)
	UpdateThemePreference(ctx context.Context, userID, themePreference string) (auth.User, error)
	UpdateAvatar(ctx context.Context, userID string, input social.AvatarUploadInput) (auth.User, error)
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

func (h SocialHandler) HandleMyPosts(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	posts, err := h.service.MyPosts(r.Context(), currentUser.ID)
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"posts": posts,
	})
}

func (h SocialHandler) HandleUserProfile(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, err := h.currentUserIfAvailable(r)
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	viewerID := ""
	if currentUser != nil {
		viewerID = currentUser.ID
	}

	result, err := h.service.ProfileByHandle(r.Context(), viewerID, strings.TrimSpace(r.PathValue("handle")))
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"profile": result.Profile,
		"posts":   result.Posts,
	})
}

func (h SocialHandler) HandleConversations(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	conversations, err := h.service.Conversations(r.Context(), currentUser.ID)
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"conversations": conversations,
	})
}

func (h SocialHandler) HandleConversation(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	thread, err := h.service.Conversation(r.Context(), currentUser.ID, strings.TrimSpace(r.PathValue("userID")))
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"user":     thread.User,
		"messages": thread.Messages,
	})
}

func (h SocialHandler) HandleSendPrivateMessage(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	var payload struct {
		Body string `json:"body"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, stdlibhttp.StatusBadRequest, err.Error(), nil)
		return
	}

	message, err := h.service.SendPrivateMessage(
		r.Context(),
		*currentUser,
		strings.TrimSpace(r.PathValue("userID")),
		social.SendPrivateMessageInput{
			Body: payload.Body,
		},
	)
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusCreated, map[string]any{
		"message": message,
	})
}

func (h SocialHandler) HandleMarkConversationRead(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	result, err := h.service.MarkConversationRead(r.Context(), currentUser.ID, strings.TrimSpace(r.PathValue("userID")))
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"conversation": result,
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

func (h SocialHandler) HandleUpdatePost(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	var payload struct {
		Title   string `json:"title"`
		Body    string `json:"body"`
		Privacy string `json:"privacy"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, stdlibhttp.StatusBadRequest, err.Error(), nil)
		return
	}

	post, err := h.service.UpdatePost(
		r.Context(),
		*currentUser,
		strings.TrimSpace(r.PathValue("postID")),
		social.UpdatePostInput{
			Title:   payload.Title,
			Body:    payload.Body,
			Privacy: payload.Privacy,
		},
	)
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"post": post,
	})
}

func (h SocialHandler) HandleDeletePost(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	if err := h.service.DeletePost(r.Context(), *currentUser, strings.TrimSpace(r.PathValue("postID"))); err != nil {
		h.handleSocialError(w, err)
		return
	}

	w.WriteHeader(stdlibhttp.StatusNoContent)
}

func (h SocialHandler) HandleUpdateAvatar(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	if err := r.ParseMultipartForm(16 << 20); err != nil {
		writeError(w, stdlibhttp.StatusBadRequest, "Avatar form data is invalid.", nil)
		return
	}

	var avatar *multipart.FileHeader
	if r.MultipartForm != nil {
		files := r.MultipartForm.File["avatar"]
		if len(files) > 0 {
			avatar = files[0]
		}
	}

	user, err := h.service.UpdateAvatar(r.Context(), currentUser.ID, social.AvatarUploadInput{
		Avatar: avatar,
	})
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"user": user,
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

func (h SocialHandler) HandleUpdateComment(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	var payload struct {
		Body string `json:"body"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, stdlibhttp.StatusBadRequest, err.Error(), nil)
		return
	}

	comment, err := h.service.UpdateComment(
		r.Context(),
		*currentUser,
		strings.TrimSpace(r.PathValue("postID")),
		strings.TrimSpace(r.PathValue("commentID")),
		social.UpdateCommentInput{
			Body: payload.Body,
		},
	)
	if err != nil {
		h.handleSocialError(w, err)
		return
	}

	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
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

func (h SocialHandler) HandleUnfollowUser(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	result, err := h.service.UnfollowUser(r.Context(), currentUser.ID, strings.TrimSpace(r.PathValue("userID")))
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

func (h SocialHandler) HandleUpdateProfile(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	currentUser, ok := h.requireCurrentUser(w, r)
	if !ok {
		return
	}

	var payload struct {
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		AboutMe   string `json:"aboutMe"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, stdlibhttp.StatusBadRequest, err.Error(), nil)
		return
	}

	user, err := h.service.UpdateProfile(r.Context(), currentUser.ID, social.UpdateProfileInput{
		FirstName: payload.FirstName,
		LastName:  payload.LastName,
		AboutMe:   payload.AboutMe,
	})
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

func (h SocialHandler) currentUserIfAvailable(r *stdlibhttp.Request) (*auth.User, error) {
	user, err := h.auth.CurrentUser(r.Context(), h.sessionIDFromRequest(r))
	if err != nil {
		if errors.Is(err, auth.ErrUnauthorized) {
			return nil, nil
		}

		return nil, err
	}

	return user, nil
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
