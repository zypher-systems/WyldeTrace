package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
)

// Version is set at build time via -ldflags when available.
var Version = "dev"

func (a *API) Healthz(w http.ResponseWriter, r *http.Request) {
	dbOK := false
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if a.Store != nil && a.Store.Pool != nil {
		if err := a.Store.Pool.Ping(ctx); err == nil {
			dbOK = true
		}
	}
	status := "ok"
	code := http.StatusOK
	if !dbOK {
		status = "degraded"
		code = http.StatusServiceUnavailable
	}
	httpx.JSON(w, code, map[string]any{
		"status":  status,
		"version": Version,
		"db":      dbOK,
		"time":    time.Now().UTC(),
	})
}

func (a *API) InstanceInfo(w http.ResponseWriter, r *http.Request) {
	dbOK := false
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if a.Store != nil && a.Store.Pool != nil {
		if err := a.Store.Pool.Ping(ctx); err == nil {
			dbOK = true
		}
	}
	mig := ""
	if a.Store != nil && a.Store.Pool != nil {
		var v int64
		if err := a.Store.Pool.QueryRow(ctx, `SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1`).Scan(&v); err == nil {
			mig = fmt.Sprintf("%d", v)
		}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"version":            Version,
		"db_ok":              dbOK,
		"migration_version":  mig,
		"login_rate_limit":   "8 fails / 15m per IP",
		"webhook_mute_hours": func() int { h, _ := a.webhookMuteHours(r.Context()); return h }(),
	})
}
