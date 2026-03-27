package handlers

import (
	"errors"
	stdlibhttp "net/http"
	"strings"

	"github.com/gorilla/websocket"

	"social-network/backend/internal/auth"
	"social-network/backend/internal/config"
	"social-network/backend/internal/realtime"
)

type WebSocketHandler struct {
	auth     authService
	session  config.SessionConfig
	hub      *realtime.Hub
	upgrader websocket.Upgrader
}

func NewWebSocketHandler(
	authService authService,
	session config.SessionConfig,
	cors config.CORSConfig,
	hub *realtime.Hub,
) WebSocketHandler {
	allowedOrigins := make(map[string]struct{}, len(cors.AllowedOrigins))
	for _, origin := range cors.AllowedOrigins {
		allowedOrigins[strings.TrimSpace(origin)] = struct{}{}
	}

	return WebSocketHandler{
		auth:    authService,
		session: session,
		hub:     hub,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *stdlibhttp.Request) bool {
				origin := strings.TrimSpace(r.Header.Get("Origin"))
				if origin == "" {
					return true
				}

				_, ok := allowedOrigins[origin]
				return ok
			},
		},
	}
}

func (h WebSocketHandler) HandleConnect(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	user, err := h.auth.CurrentUser(r.Context(), h.sessionIDFromRequest(r))
	if err != nil {
		h.handleConnectError(w, err)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	h.hub.ServeConn(user.ID, conn)
}

func (h WebSocketHandler) sessionIDFromRequest(r *stdlibhttp.Request) string {
	cookie, err := r.Cookie(h.session.CookieName)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(cookie.Value)
}

func (h WebSocketHandler) handleConnectError(w stdlibhttp.ResponseWriter, err error) {
	switch {
	case err == nil:
		return
	case errors.Is(err, auth.ErrUnauthorized):
		writeError(w, stdlibhttp.StatusUnauthorized, "Authentication required.", nil)
	default:
		writeError(w, stdlibhttp.StatusInternalServerError, "Could not open the realtime connection.", nil)
	}
}
