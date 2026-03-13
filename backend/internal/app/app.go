package app

import (
	"database/sql"
	"net"
	stdlibhttp "net/http"

	"social-network/backend/internal/config"
	transporthttp "social-network/backend/internal/http"
	"social-network/backend/pkg/db/postgres"
)

type App struct {
	cfg    config.Config
	router stdlibhttp.Handler
	db     *sql.DB
}

func New() (*App, error) {
	cfg := config.Load()

	db, err := postgres.Open(cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	if err := postgres.ApplyMigrations(db, cfg.MigrationsDir); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &App{
		cfg:    cfg,
		router: transporthttp.NewRouter(cfg, db),
		db:     db,
	}, nil
}

func (a *App) Address() string {
	return net.JoinHostPort(a.cfg.Host, a.cfg.Port)
}

func (a *App) Router() stdlibhttp.Handler {
	return a.router
}

func (a *App) Close() error {
	if a.db == nil {
		return nil
	}

	return a.db.Close()
}
