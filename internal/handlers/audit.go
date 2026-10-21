package handlers

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/wyldetechnologies/wyldetrace/internal/db"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
	"github.com/wyldetechnologies/wyldetrace/internal/middleware"
)

type auditDTO struct {
	ID         uuid.UUID       `json:"id"`
	ActorID    *uuid.UUID      `json:"actor_id"`
	ActorEmail string          `json:"actor_email"`
	Action     string          `json:"action"`
	EntityType string          `json:"entity_type"`
	EntityID   string          `json:"entity_id"`
	Details    json.RawMessage `json:"details"`
	CreatedAt  time.Time       `json:"created_at"`
}

func (a *API) audit(ctx context.Context, action, entityType, entityID string, details any) {
	actorID := pgtype.UUID{}
	email := ""
	if claims, ok := middleware.ClaimsFromContext(ctx); ok {
		actorID = db.UUIDToNull(claims.UserID)
		email = claims.Email
	}
	raw, _ := json.Marshal(details)
	if raw == nil {
		raw = []byte("{}")
	}
	if _, err := a.Store.Queries.CreateAuditLog(ctx, sqlcdb.CreateAuditLogParams{
		ActorID:    actorID,
		ActorEmail: email,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Details:    raw,
	}); err != nil {
		log.Printf("audit: %v", err)
	}
}

type auditFilter struct {
	Action string
	Actor  string
	FromTs pgtype.Timestamptz
	ToTs   pgtype.Timestamptz
}

func parseAuditFilter(r *http.Request) auditFilter {
	f := auditFilter{
		Action: strings.TrimSpace(r.URL.Query().Get("action")),
		Actor:  strings.TrimSpace(r.URL.Query().Get("actor")),
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("from")); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			f.FromTs = pgtype.Timestamptz{Time: t.UTC(), Valid: true}
		} else if t, err := time.Parse("2006-01-02", raw); err == nil {
			f.FromTs = pgtype.Timestamptz{Time: t.UTC(), Valid: true}
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("to")); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			f.ToTs = pgtype.Timestamptz{Time: t.UTC(), Valid: true}
		} else if t, err := time.Parse("2006-01-02", raw); err == nil {
			// inclusive end-of-day
			f.ToTs = pgtype.Timestamptz{Time: t.UTC().Add(24*time.Hour - time.Nanosecond), Valid: true}
		}
	}
	return f
}

func (a *API) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	pq := httpx.ParsePage(r)
	sq := httpx.ParseSort(r, map[string]bool{
		"created_at": true, "action": true, "actor_email": true, "entity_type": true,
	}, "created_at", "desc")
	f := parseAuditFilter(r)
	total, err := a.Store.Queries.CountAuditLogs(r.Context(), sqlcdb.CountAuditLogsParams{
		Action: f.Action, Actor: f.Actor, FromTs: f.FromTs, ToTs: f.ToTs,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	items, err := a.Store.Queries.ListAuditLogsPaged(r.Context(), sqlcdb.ListAuditLogsPagedParams{
		Action: f.Action, Actor: f.Actor, FromTs: f.FromTs, ToTs: f.ToTs,
		SortBy: sq.By, SortDir: sq.Dir, RowOffset: pq.Offset, RowLimit: pq.PageSize,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]auditDTO, 0, len(items))
	for _, item := range items {
		dto := auditDTO{
			ID:         item.ID,
			ActorEmail: item.ActorEmail,
			Action:     item.Action,
			EntityType: item.EntityType,
			EntityID:   item.EntityID,
			Details:    item.Details,
			CreatedAt:  item.CreatedAt.Time,
		}
		dto.ActorID = db.NullUUID(item.ActorID)
		out = append(out, dto)
	}
	httpx.JSON(w, http.StatusOK, httpx.PageResult[auditDTO]{
		Items: out, Total: total, Page: pq.Page, PageSize: pq.PageSize,
	})
}

func (a *API) ExportAuditLogsCSV(w http.ResponseWriter, r *http.Request) {
	limit := int32(10000)
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 100000 {
			limit = int32(n)
		}
	}
	f := parseAuditFilter(r)
	items, err := a.Store.Queries.ListAuditLogsExport(r.Context(), sqlcdb.ListAuditLogsExportParams{
		Action: f.Action, Actor: f.Actor, FromTs: f.FromTs, ToTs: f.ToTs, RowLimit: limit,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="audit-logs.csv"`)
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"id", "created_at", "actor_email", "actor_id", "action", "entity_type", "entity_id", "details"})
	for _, item := range items {
		actorID := ""
		if id := db.NullUUID(item.ActorID); id != nil {
			actorID = id.String()
		}
		details := string(item.Details)
		if details == "" {
			details = "{}"
		}
		_ = cw.Write([]string{
			item.ID.String(),
			item.CreatedAt.Time.UTC().Format(time.RFC3339),
			item.ActorEmail,
			actorID,
			item.Action,
			item.EntityType,
			item.EntityID,
			details,
		})
	}
	cw.Flush()
	a.audit(r.Context(), "audit.export", "audit_log", "", map[string]any{"rows": len(items)})
}
