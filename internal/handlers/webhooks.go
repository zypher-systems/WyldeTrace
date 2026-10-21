package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/wyldetechnologies/wyldetrace/internal/db"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
	"github.com/wyldetechnologies/wyldetrace/internal/httpx"
	"github.com/wyldetechnologies/wyldetrace/internal/webhook"
)

type webhookDTO struct {
	ID         uuid.UUID  `json:"id"`
	Name       string     `json:"name"`
	URL        string     `json:"url"`
	SecretSet  bool       `json:"secret_set"`
	Events     []string   `json:"events"`
	Format     string     `json:"format"`
	Enabled    bool       `json:"enabled"`
	LastStatus string     `json:"last_status"`
	LastError  string     `json:"last_error"`
	LastSentAt *time.Time `json:"last_sent_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type webhookRequest struct {
	Name    string   `json:"name"`
	URL     string   `json:"url"`
	Secret  *string  `json:"secret"`
	Events  []string `json:"events"`
	Format  *string  `json:"format"`
	Enabled *bool    `json:"enabled"`
}

func toWebhookDTO(w sqlcdb.Webhook) webhookDTO {
	return webhookDTO{
		ID: w.ID, Name: w.Name, URL: w.Url, SecretSet: w.Secret != "",
		Events: splitEvents(w.Events), Format: webhook.NormalizeFormat(w.Format), Enabled: w.Enabled,
		LastStatus: w.LastStatus, LastError: w.LastError,
		LastSentAt: timestamptzPtr(w.LastSentAt),
		CreatedAt:  w.CreatedAt.Time, UpdatedAt: w.UpdatedAt.Time,
	}
}

func splitEvents(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{webhook.EventDiscoveryNewAlive, webhook.EventDiscoveryStale}
	}
	return out
}

func joinEvents(events []string) string {
	if len(events) == 0 {
		return webhook.EventDiscoveryNewAlive + "," + webhook.EventDiscoveryStale
	}
	clean := make([]string, 0, len(events))
	for _, e := range events {
		e = strings.TrimSpace(e)
		if e != "" {
			clean = append(clean, e)
		}
	}
	return strings.Join(clean, ",")
}

func (a *API) ListWebhooks(w http.ResponseWriter, r *http.Request) {
	items, err := a.Store.Queries.ListWebhooks(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]webhookDTO, 0, len(items))
	for _, item := range items {
		out = append(out, toWebhookDTO(item))
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (a *API) CreateWebhook(w http.ResponseWriter, r *http.Request) {
	var req webhookRequest
	if err := httpx.Decode(r, &req); err != nil || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.URL) == "" {
		httpx.Error(w, http.StatusBadRequest, "name and url required")
		return
	}
	if err := a.validateWebhookOutbound(req.URL); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	secret := ""
	if req.Secret != nil {
		secret = *req.Secret
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	format := webhook.FormatJSON
	if req.Format != nil {
		format = webhook.NormalizeFormat(*req.Format)
		if !webhook.ValidFormat(format) {
			httpx.Error(w, http.StatusBadRequest, "format must be json, slack, or teams")
			return
		}
	}
	item, err := a.Store.Queries.CreateWebhook(r.Context(), sqlcdb.CreateWebhookParams{
		Name: req.Name, Url: req.URL, Secret: secret, Events: joinEvents(req.Events), Enabled: enabled, Format: format,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	a.audit(r.Context(), "webhook.create", "webhook", item.ID.String(), map[string]any{"name": item.Name, "url": item.Url, "format": format})
	httpx.JSON(w, http.StatusCreated, toWebhookDTO(item))
}

func (a *API) UpdateWebhook(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	existing, err := a.Store.Queries.GetWebhook(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "webhook not found")
		return
	}
	var req webhookRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = existing.Name
	}
	u := strings.TrimSpace(req.URL)
	if u == "" {
		u = existing.Url
	}
	if err := a.validateWebhookOutbound(u); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	secret := existing.Secret
	if req.Secret != nil {
		secret = *req.Secret
	}
	events := existing.Events
	if req.Events != nil {
		events = joinEvents(req.Events)
	}
	enabled := existing.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	format := webhook.NormalizeFormat(existing.Format)
	if req.Format != nil {
		format = webhook.NormalizeFormat(*req.Format)
		if !webhook.ValidFormat(format) {
			httpx.Error(w, http.StatusBadRequest, "format must be json, slack, or teams")
			return
		}
	}
	item, err := a.Store.Queries.UpdateWebhook(r.Context(), sqlcdb.UpdateWebhookParams{
		ID: id, Name: name, Url: u, Secret: secret, Events: events, Enabled: enabled, Format: format,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	a.audit(r.Context(), "webhook.update", "webhook", item.ID.String(), map[string]any{"name": item.Name, "format": format})
	httpx.JSON(w, http.StatusOK, toWebhookDTO(item))
}

func (a *API) DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := a.Store.Queries.DeleteWebhook(r.Context(), id); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	a.audit(r.Context(), "webhook.delete", "webhook", id.String(), map[string]any{})
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) TestWebhook(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if a.Hooks == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "webhooks unavailable")
		return
	}
	if err := a.Hooks.DeliverOne(r.Context(), id, webhook.EventDiscoveryNewAlive, map[string]any{
		"test": true, "webhook_id": id.String(), "message": "WyldeTrace webhook test",
	}); err != nil {
		httpx.Error(w, http.StatusNotFound, "webhook not found")
		return
	}
	item, _ := a.Store.Queries.GetWebhook(r.Context(), id)
	httpx.JSON(w, http.StatusOK, toWebhookDTO(item))
}

type previewWebhookReq struct {
	Format  string         `json:"format"`
	Event   string         `json:"event"`
	Payload map[string]any `json:"payload"`
}

// PreviewWebhook renders a sample delivery body without sending it.
// POST /webhooks/preview uses the request format; POST /webhooks/{id}/preview uses the stored webhook format.
func (a *API) PreviewWebhook(w http.ResponseWriter, r *http.Request) {
	format := webhook.FormatJSON
	event := webhook.EventDiscoveryNewAlive
	var payload map[string]any

	if rawID := chiURLParam(r, "id"); rawID != "" {
		id, err := uuid.Parse(rawID)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "invalid id")
			return
		}
		item, err := a.Store.Queries.GetWebhook(r.Context(), id)
		if err != nil {
			httpx.Error(w, http.StatusNotFound, "webhook not found")
			return
		}
		format = webhook.NormalizeFormat(item.Format)
	}

	var req previewWebhookReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		httpx.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	if chiURLParam(r, "id") == "" && req.Format != "" {
		format = webhook.NormalizeFormat(req.Format)
		if !webhook.ValidFormat(format) {
			httpx.Error(w, http.StatusBadRequest, "format must be json, slack, or teams")
			return
		}
	}
	if strings.TrimSpace(req.Event) != "" {
		event = strings.TrimSpace(req.Event)
	}
	if req.Payload != nil {
		payload = req.Payload
	} else {
		payload = sampleWebhookPayload(event)
	}

	env := webhook.Envelope{Event: event, Timestamp: time.Now().UTC(), Payload: payload}
	body, err := webhook.EncodeBody(format, env)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	var parsed any
	_ = json.Unmarshal(body, &parsed)
	httpx.JSON(w, http.StatusOK, map[string]any{
		"format":  format,
		"event":   event,
		"payload": parsed,
		"raw":     string(body),
	})
}

func sampleWebhookPayload(event string) map[string]any {
	switch event {
	case webhook.EventDiscoveryStale:
		return map[string]any{
			"test": true, "count": 2, "message": "Sample stale hosts",
			"hosts": []string{"10.0.0.10", "10.0.0.11"},
		}
	case webhook.EventDiscoveryError:
		return map[string]any{"test": true, "error": "sample sweep timeout", "job": "Hourly + enrich"}
	case webhook.EventIPDeleted:
		return map[string]any{"test": true, "address": "10.0.0.50", "hostname": "old-host"}
	case webhook.EventIPClaimed:
		return map[string]any{"test": true, "address": "10.0.0.51", "hostname": "new-host"}
	case webhook.EventSubnetUtilHigh:
		return map[string]any{"test": true, "cidr": "10.0.0.0/24", "percent": 92.5, "used": 236, "total": 254}
	case webhook.EventReportsSummary:
		return map[string]any{
			"test": true, "subnets": 12, "ips": 480, "inbox_open": 3,
			"message": "Sample reports.summary payload",
		}
	default:
		return map[string]any{
			"test": true, "count": 1, "message": "WyldeTrace webhook preview",
			"hosts": []string{"10.0.0.12"},
		}
	}
}

type webhookDeliveryDTO struct {
	ID         uuid.UUID `json:"id"`
	WebhookID  uuid.UUID `json:"webhook_id"`
	Event      string    `json:"event"`
	Status     string    `json:"status"`
	StatusCode int32     `json:"status_code"`
	Error      string    `json:"error"`
	CreatedAt  time.Time `json:"created_at"`
}

func toWebhookDeliveryDTO(d sqlcdb.WebhookDelivery) webhookDeliveryDTO {
	return webhookDeliveryDTO{
		ID: d.ID, WebhookID: d.WebhookID, Event: d.Event,
		Status: d.Status, StatusCode: d.StatusCode, Error: d.Error,
		CreatedAt: d.CreatedAt.Time,
	}
}

func (a *API) ListWebhookDeliveries(w http.ResponseWriter, r *http.Request) {
	pq := httpx.ParsePage(r)
	var webhookID pgtype.UUID
	if raw := chiURLParam(r, "id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "invalid webhook id")
			return
		}
		webhookID = db.UUIDToNull(id)
	} else if raw := r.URL.Query().Get("webhook_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "invalid webhook_id")
			return
		}
		webhookID = db.UUIDToNull(id)
	}
	total, err := a.Store.Queries.CountWebhookDeliveryLogs(r.Context(), webhookID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	items, err := a.Store.Queries.ListWebhookDeliveryLogs(r.Context(), sqlcdb.ListWebhookDeliveryLogsParams{
		WebhookID: webhookID, RowLimit: pq.PageSize, RowOffset: pq.Offset,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]webhookDeliveryDTO, 0, len(items))
	for _, item := range items {
		out = append(out, toWebhookDeliveryDTO(item))
	}
	httpx.JSON(w, http.StatusOK, httpx.PageResult[webhookDeliveryDTO]{
		Items: out, Total: total, Page: pq.Page, PageSize: pq.PageSize,
	})
}

func (a *API) RetryWebhookDelivery(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chiURLParam(r, "deliveryID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid delivery id")
		return
	}
	if a.Hooks == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "webhooks unavailable")
		return
	}
	if err := a.Hooks.RetryDelivery(r.Context(), id); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	a.audit(r.Context(), "webhook.retry", "webhook_delivery", id.String(), map[string]any{})
	httpx.JSON(w, http.StatusOK, map[string]any{"status": "ok", "delivery_id": id.String()})
}
