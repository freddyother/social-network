package transporthttp

import (
	"database/sql"
	stdlibhttp "net/http"

	"social-network/backend/internal/auth"
	"social-network/backend/internal/config"
	"social-network/backend/internal/http/handlers"
	"social-network/backend/internal/http/middleware"
	"social-network/backend/internal/http/response"
	"social-network/backend/internal/social"
)

func NewRouter(cfg config.Config, db *sql.DB) stdlibhttp.Handler {
	mux := stdlibhttp.NewServeMux()

	healthHandler := handlers.NewHealthHandler(db)
	metaHandler := handlers.NewMetaHandler(cfg)
	authService := auth.NewService(db, cfg.Session.TTL)
	authHandler := handlers.NewAuthHandler(authService, cfg.Session)
	socialHandler := handlers.NewSocialHandler(
		authService,
		social.NewService(db, cfg.UploadsDir, cfg.PublicBaseURL),
		cfg.Session,
	)

	mux.HandleFunc("GET /api/v1/health", healthHandler.Handle)
	mux.HandleFunc("GET /api/v1/meta", metaHandler.Handle)
	mux.HandleFunc("POST /api/v1/auth/register", authHandler.HandleRegister)
	mux.HandleFunc("POST /api/v1/auth/login", authHandler.HandleLogin)
	mux.HandleFunc("POST /api/v1/auth/logout", authHandler.HandleLogout)
	mux.HandleFunc("GET /api/v1/auth/me", authHandler.HandleCurrentUser)
	mux.HandleFunc("GET /api/v1/posts", socialHandler.HandleFeed)
	mux.HandleFunc("POST /api/v1/posts", socialHandler.HandleCreatePost)
	mux.HandleFunc("GET /api/v1/posts/{postID}/comments", socialHandler.HandleComments)
	mux.HandleFunc("POST /api/v1/posts/{postID}/comments", socialHandler.HandleCreateComment)
	mux.HandleFunc("GET /api/v1/users/discover", socialHandler.HandleDiscoverUsers)
	mux.HandleFunc("POST /api/v1/users/{userID}/follow", socialHandler.HandleFollowUser)
	mux.HandleFunc("GET /api/v1/follow-requests", socialHandler.HandleFollowRequests)
	mux.HandleFunc("POST /api/v1/follow-requests/{requestID}/accept", socialHandler.HandleAcceptFollowRequest)
	mux.HandleFunc("POST /api/v1/follow-requests/{requestID}/decline", socialHandler.HandleDeclineFollowRequest)
	mux.HandleFunc("PATCH /api/v1/users/me/profile-visibility", socialHandler.HandleUpdateProfileVisibility)
	mux.HandleFunc("GET /api/v1/notifications", socialHandler.HandleNotifications)
	mux.HandleFunc("POST /api/v1/notifications/{notificationID}/read", socialHandler.HandleMarkNotificationRead)
	mux.Handle("GET /uploads/", stdlibhttp.StripPrefix("/uploads/", stdlibhttp.FileServer(stdlibhttp.Dir(cfg.UploadsDir))))
	mux.HandleFunc("GET /", func(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
		response.JSON(w, stdlibhttp.StatusOK, map[string]any{
			"name":        "social-network",
			"message":     "Go backend running",
			"frontend":    "Vue 3 SPA",
			"api_version": "v1",
		})
	})

	return middleware.CORS(cfg.CORS)(mux)
}
