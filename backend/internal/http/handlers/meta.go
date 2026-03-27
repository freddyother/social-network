package handlers

import (
	stdlibhttp "net/http"

	"social-network/backend/internal/config"
	"social-network/backend/internal/http/response"
)

type MetaHandler struct {
	cfg config.Config
}

func NewMetaHandler(cfg config.Config) MetaHandler {
	return MetaHandler{cfg: cfg}
}

func (h MetaHandler) Handle(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	response.JSON(w, stdlibhttp.StatusOK, map[string]any{
		"name":       "NEXO",
		"stack":      []string{"go", "postgresql", "vue-3"},
		"app_env":    h.cfg.AppEnv,
		"api_prefix": "/api/v1",
		"modules": []string{
			"auth",
			"profiles",
			"followers",
			"posts",
			"groups",
			"notifications",
			"private-chat",
			"group-chat",
		},
		"cors": map[string]any{
			"allowed_origins": h.cfg.CORS.AllowedOrigins,
			"credentials":     h.cfg.CORS.AllowCredentials,
		},
	})
}
