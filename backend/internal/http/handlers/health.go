package handlers

import (
	"context"
	"database/sql"
	stdlibhttp "net/http"
	"time"

	"social-network/backend/internal/http/response"
)

type HealthHandler struct {
	db *sql.DB
}

func NewHealthHandler(db *sql.DB) HealthHandler {
	return HealthHandler{db: db}
}

func (h HealthHandler) Handle(w stdlibhttp.ResponseWriter, r *stdlibhttp.Request) {
	status := "up"
	databaseStatus := "up"
	httpStatus := stdlibhttp.StatusOK

	if h.db == nil {
		status = "degraded"
		databaseStatus = "missing"
		httpStatus = stdlibhttp.StatusServiceUnavailable
	} else {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := h.db.PingContext(ctx); err != nil {
			status = "degraded"
			databaseStatus = err.Error()
			httpStatus = stdlibhttp.StatusServiceUnavailable
		}
	}

	response.JSON(w, httpStatus, map[string]any{
		"status": status,
		"services": map[string]string{
			"api":      "up",
			"database": databaseStatus,
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}
