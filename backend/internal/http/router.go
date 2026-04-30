package transporthttp

import (
	"context"
	"database/sql"
	stdlibhttp "net/http"

	"social-network/backend/internal/auth"
	"social-network/backend/internal/config"
	"social-network/backend/internal/http/handlers"
	"social-network/backend/internal/http/middleware"
	"social-network/backend/internal/http/response"
	"social-network/backend/internal/realtime"
	"social-network/backend/internal/social"
)

func NewRouter(cfg config.Config, db *sql.DB) stdlibhttp.Handler {
	mux := stdlibhttp.NewServeMux()

	healthHandler := handlers.NewHealthHandler(db)
	metaHandler := handlers.NewMetaHandler(cfg)
	resetMailer := auth.NewSMTPPasswordResetMailer(auth.SMTPMailerConfig{
		Host:      cfg.Mail.SMTPHost,
		Port:      cfg.Mail.SMTPPort,
		Username:  cfg.Mail.Username,
		Password:  cfg.Mail.Password,
		FromEmail: cfg.Mail.FromEmail,
		FromName:  cfg.Mail.FromName,
	})
	authService := auth.NewService(db, cfg.Session.TTL, auth.PasswordResetConfig{
		TokenTTL:             cfg.PasswordReset.TokenTTL,
		ResetURL:             cfg.PasswordReset.URL,
		RevealLinkInResponse: cfg.PasswordReset.RevealLinkInResponse,
	}, resetMailer)
	realtimeHub := realtime.NewHub()
	socialService := social.NewService(db, cfg.UploadsDir, cfg.PublicBaseURL, realtimeHub)
	realtimeHub.SetPostSubscriptionAuthorizer(socialService.CanViewPost)
	realtimeHub.SetUserConnectedHandler(func(ctx context.Context, userID string) {
		_ = socialService.MarkUndeliveredMessagesDelivered(ctx, userID)
	})
	realtimeHub.SetMessageDeliveredHandler(func(ctx context.Context, userID, messageID string) {
		_ = socialService.MarkMessageDelivered(ctx, userID, messageID)
	})
	realtimeHub.SetConversationReadHandler(func(ctx context.Context, userID, conversationUserID string) {
		_, _ = socialService.MarkConversationRead(ctx, userID, conversationUserID)
	})
	realtimeHub.SetChatHistoryHandler(func(ctx context.Context, userID, conversationUserID, beforeMessageID string, limit int) (any, error) {
		return socialService.ConversationHistory(ctx, userID, conversationUserID, beforeMessageID, limit)
	})
	authHandler := handlers.NewAuthHandler(authService, cfg.Session)
	socialHandler := handlers.NewSocialHandler(
		authService,
		socialService,
		cfg.Session,
	)
	wsHandler := handlers.NewWebSocketHandler(authService, cfg.Session, cfg.CORS, realtimeHub)

	mux.HandleFunc("GET /api/v1/health", healthHandler.Handle)
	mux.HandleFunc("GET /api/v1/meta", metaHandler.Handle)
	mux.HandleFunc("POST /api/v1/auth/register", authHandler.HandleRegister)
	mux.HandleFunc("POST /api/v1/auth/login", authHandler.HandleLogin)
	mux.HandleFunc("GET /api/v1/auth/nickname-availability", authHandler.HandleNicknameAvailability)
	mux.HandleFunc("POST /api/v1/auth/forgot-password", authHandler.HandleForgotPassword)
	mux.HandleFunc("POST /api/v1/auth/reset-password", authHandler.HandleResetPassword)
	mux.HandleFunc("POST /api/v1/auth/logout", authHandler.HandleLogout)
	mux.HandleFunc("GET /api/v1/auth/me", authHandler.HandleCurrentUser)
	mux.HandleFunc("GET /api/v1/ws", wsHandler.HandleConnect)
	mux.HandleFunc("GET /api/v1/posts", socialHandler.HandleFeed)
	mux.HandleFunc("GET /api/v1/posts/mine", socialHandler.HandleMyPosts)
	mux.HandleFunc("GET /api/v1/posts/{postID}", socialHandler.HandlePost)
	mux.HandleFunc("POST /api/v1/posts", socialHandler.HandleCreatePost)
	mux.HandleFunc("PATCH /api/v1/posts/{postID}", socialHandler.HandleUpdatePost)
	mux.HandleFunc("DELETE /api/v1/posts/{postID}", socialHandler.HandleDeletePost)
	mux.HandleFunc("GET /api/v1/users/{handle}", socialHandler.HandleUserProfile)
	mux.HandleFunc("GET /api/v1/groups", socialHandler.HandleGroups)
	mux.HandleFunc("POST /api/v1/groups", socialHandler.HandleCreateGroup)
	mux.HandleFunc("GET /api/v1/groups/{groupID}", socialHandler.HandleGroup)
	mux.HandleFunc("POST /api/v1/groups/{groupID}/join", socialHandler.HandleJoinGroup)
	mux.HandleFunc("GET /api/v1/groups/{groupID}/join-requests", socialHandler.HandleGroupJoinRequests)
	mux.HandleFunc("POST /api/v1/groups/{groupID}/join-requests/{requestID}/accept", socialHandler.HandleAcceptGroupJoinRequest)
	mux.HandleFunc("POST /api/v1/groups/{groupID}/join-requests/{requestID}/decline", socialHandler.HandleDeclineGroupJoinRequest)
	mux.HandleFunc("GET /api/v1/groups/{groupID}/posts", socialHandler.HandleGroupPosts)
	mux.HandleFunc("POST /api/v1/groups/{groupID}/posts", socialHandler.HandleCreateGroupPost)
	mux.HandleFunc("GET /api/v1/groups/{groupID}/posts/{postID}/comments", socialHandler.HandleGroupComments)
	mux.HandleFunc("POST /api/v1/groups/{groupID}/posts/{postID}/comments", socialHandler.HandleCreateGroupComment)
	mux.HandleFunc("GET /api/v1/groups/{groupID}/events", socialHandler.HandleGroupEvents)
	mux.HandleFunc("POST /api/v1/groups/{groupID}/events", socialHandler.HandleCreateGroupEvent)
	mux.HandleFunc("POST /api/v1/groups/{groupID}/events/{eventID}/respond", socialHandler.HandleRespondGroupEvent)
	mux.HandleFunc("GET /api/v1/groups/{groupID}/messages", socialHandler.HandleGroupMessages)
	mux.HandleFunc("POST /api/v1/groups/{groupID}/messages", socialHandler.HandleSendGroupMessage)
	mux.HandleFunc("GET /api/v1/groups/{groupID}/invite-candidates", socialHandler.HandleGroupInviteCandidates)
	mux.HandleFunc("POST /api/v1/groups/{groupID}/invite", socialHandler.HandleInviteUserToGroup)
	mux.HandleFunc("GET /api/v1/search", socialHandler.HandleSearch)
	mux.HandleFunc("GET /api/v1/chat/conversations", socialHandler.HandleConversations)
	mux.HandleFunc("GET /api/v1/chat/conversations/{userID}/messages", socialHandler.HandleConversation)
	mux.HandleFunc("POST /api/v1/chat/conversations/{userID}/messages", socialHandler.HandleSendPrivateMessage)
	mux.HandleFunc("POST /api/v1/chat/conversations/{userID}/read", socialHandler.HandleMarkConversationRead)
	mux.HandleFunc("POST /api/v1/users/me/avatar", socialHandler.HandleUpdateAvatar)
	mux.HandleFunc("GET /api/v1/posts/{postID}/comments", socialHandler.HandleComments)
	mux.HandleFunc("POST /api/v1/posts/{postID}/comments", socialHandler.HandleCreateComment)
	mux.HandleFunc("PATCH /api/v1/posts/{postID}/comments/{commentID}", socialHandler.HandleUpdateComment)
	mux.HandleFunc("GET /api/v1/users/discover", socialHandler.HandleDiscoverUsers)
	mux.HandleFunc("POST /api/v1/users/{userID}/follow", socialHandler.HandleFollowUser)
	mux.HandleFunc("DELETE /api/v1/users/{userID}/follow", socialHandler.HandleUnfollowUser)
	mux.HandleFunc("GET /api/v1/follow-requests", socialHandler.HandleFollowRequests)
	mux.HandleFunc("POST /api/v1/follow-requests/{requestID}/accept", socialHandler.HandleAcceptFollowRequest)
	mux.HandleFunc("POST /api/v1/follow-requests/{requestID}/decline", socialHandler.HandleDeclineFollowRequest)
	mux.HandleFunc("PATCH /api/v1/users/me/profile", socialHandler.HandleUpdateProfile)
	mux.HandleFunc("PATCH /api/v1/users/me/profile-visibility", socialHandler.HandleUpdateProfileVisibility)
	mux.HandleFunc("PATCH /api/v1/users/me/theme-preference", socialHandler.HandleUpdateThemePreference)
	mux.HandleFunc("GET /api/v1/notifications", socialHandler.HandleNotifications)
	mux.HandleFunc("POST /api/v1/notifications/{notificationID}/read", socialHandler.HandleMarkNotificationRead)
	mux.Handle("GET /uploads/", stdlibhttp.StripPrefix("/uploads/", stdlibhttp.FileServer(stdlibhttp.Dir(cfg.UploadsDir))))
	mux.HandleFunc("GET /", func(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
		response.JSON(w, stdlibhttp.StatusOK, map[string]any{
			"name":        "NEXO",
			"message":     "Go backend running",
			"frontend":    "Vue 3 SPA",
			"api_version": "v1",
		})
	})

	return middleware.CORS(cfg.CORS)(mux)
}
