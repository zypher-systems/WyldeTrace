package handlers

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
)

type notificationDTO struct {
	ID         uuid.UUID  `json:"id"`
	Kind       string     `json:"kind"`
	Title      string     `json:"title"`
	Body       string     `json:"body"`
	EntityType string     `json:"entity_type"`
	EntityID   string     `json:"entity_id"`
	CreatedAt  time.Time  `json:"created_at"`
	ReadAt     *time.Time `json:"read_at"`
}

func toNotificationDTO(n sqlcdb.AppNotification) notificationDTO {
	dto := notificationDTO{
		ID: n.ID, Kind: n.Kind, Title: n.Title, Body: n.Body,
		EntityType: n.EntityType, EntityID: n.EntityID, CreatedAt: n.CreatedAt.Time,
	}
	if n.ReadAt.Valid {
		t := n.ReadAt.Time
		dto.ReadAt = &t
	}
	return dto
}

func (a *API) ListNotifications(w http.ResponseWriter, r *http.Request) {
	pq := httpx.ParsePage(r)
	total, err := a.Store.Queries.CountNotifications(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	unread, _ := a.Store.Queries.CountUnreadNotifications(r.Context())
	items, err := a.Store.Queries.ListNotifications(r.Context(), sqlcdb.ListNotificationsParams{
		RowLimit: pq.PageSize, RowOffset: pq.Offset,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]notificationDTO, 0, len(items))
	for _, item := range items {
		out = append(out, toNotificationDTO(item))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"items": out, "total": total, "unread": unread,
		"page": pq.Page, "page_size": pq.PageSize,
	})
}

func (a *API) MarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	item, err := a.Store.Queries.MarkNotificationRead(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "notification not found")
		return
	}
	httpx.JSON(w, http.StatusOK, toNotificationDTO(item))
}

func (a *API) MarkAllNotificationsRead(w http.ResponseWriter, r *http.Request) {
	n, err := a.Store.Queries.MarkAllNotificationsRead(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"marked": n})
}
