package transporthttp

import (
	"database/sql"
	stdlibhttp "net/http"

	"social-network/backend/internal/auth"
	"social-network/backend/internal/config"
	"social-network/backend/internal/http/handlers"
	"social-network/backend/internal/http/middleware"
	"social-network/backend/internal/http/response"
)

func NewRouter(cfg config.Config, db *sql.DB) stdlibhttp.Handler {
	mux := stdlibhttp.NewServeMux()

	healthHandler := handlers.NewHealthHandler(db)
	metaHandler := handlers.NewMetaHandler(cfg)
	authHandler := handlers.NewAuthHandler(auth.NewService(db, cfg.Session.TTL), cfg.Session)

	mux.HandleFunc("GET /api/v1/health", healthHandler.Handle)
	mux.HandleFunc("GET /api/v1/meta", metaHandler.Handle)
	mux.HandleFunc("POST /api/v1/auth/register", authHandler.HandleRegister)
	mux.HandleFunc("POST /api/v1/auth/login", authHandler.HandleLogin)
	mux.HandleFunc("POST /api/v1/auth/logout", authHandler.HandleLogout)
	mux.HandleFunc("GET /api/v1/auth/me", authHandler.HandleCurrentUser)
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
