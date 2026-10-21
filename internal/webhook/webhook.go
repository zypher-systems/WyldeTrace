package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wyldetechnologies/wyldetrace/internal/db"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
	"github.com/wyldetechnologies/wyldetrace/internal/metrics"
)

const (
	EventDiscoveryNewAlive = "discovery.new_alive"
	EventDiscoveryStale    = "discovery.stale"
	EventDiscoveryError    = "discovery.error"
	EventIPDeleted         = "ip.deleted"
	EventIPClaimed         = "ip.claimed"
	EventSubnetUtilHigh    = "subnet.utilization_high"
	EventReportsSummary    = "reports.summary"
)

// KnownEvents is the documented event catalog for UI defaults.
var KnownEvents = []string{
	EventDiscoveryNewAlive,
	EventDiscoveryStale,
	EventDiscoveryError,
	EventIPDeleted,
	EventIPClaimed,
	EventSubnetUtilHigh,
	EventReportsSummary,
}

type Notifier struct {
	Store  *db.Store
	Client *http.Client
}

func New(store *db.Store) *Notifier {
	return &Notifier{
		Store:  store,
		Client: &http.Client{Timeout: 10 * time.Second},
	}
}

type Envelope struct {
	Event     string         `json:"event"`
	Timestamp time.Time      `json:"timestamp"`
	Payload   map[string]any `json:"payload"`
}

func (n *Notifier) Dispatch(ctx context.Context, event string, payload map[string]any) {
	if n == nil || n.Store == nil {
		return
	}
	hooks, err := n.Store.Queries.ListEnabledWebhooks(ctx)
	if err != nil {
		log.Printf("webhooks: list: %v", err)
		return
	}
	env := Envelope{Event: event, Timestamp: time.Now().UTC(), Payload: payload}
	for _, h := range hooks {
		if !Subscribes(h.Events, event) {
			continue
		}
		body, err := EncodeBody(h.Format, env)
		if err != nil {
			log.Printf("webhooks: encode %s: %v", h.ID, err)
			continue
		}
		hook := h
		go n.deliver(context.Background(), hook, body, event)
	}
}

func Subscribes(eventsCSV, event string) bool {
	for _, e := range strings.Split(eventsCSV, ",") {
		e = strings.TrimSpace(e)
		if e == "" || e == "*" || e == event {
			return true
		}
	}
	return false
}

func (n *Notifier) deliver(ctx context.Context, h sqlcdb.Webhook, body []byte, event string) {
	status := "error"
	statusCode := 0
	errMsg := ""

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.Url, bytes.NewReader(body))
	if err != nil {
		errMsg = err.Error()
		n.finish(ctx, h.ID, event, body, status, statusCode, errMsg)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "WyldeTrace-Webhook/1.0")
	req.Header.Set("X-WyldeTrace-Event", event)
	if h.Secret != "" {
		mac := hmac.New(sha256.New, []byte(h.Secret))
		_, _ = mac.Write(body)
		req.Header.Set("X-WyldeTrace-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
	res, err := n.Client.Do(req)
	if err != nil {
		errMsg = err.Error()
		n.finish(ctx, h.ID, event, body, status, statusCode, errMsg)
		return
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 64*1024))
	statusCode = res.StatusCode
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		status = "ok"
		errMsg = ""
	} else {
		errMsg = fmt.Sprintf("HTTP %d", res.StatusCode)
	}
	n.finish(ctx, h.ID, event, body, status, statusCode, errMsg)
}

func (n *Notifier) finish(ctx context.Context, webhookID uuid.UUID, event string, body []byte, status string, statusCode int, errMsg string) {
	if status == "error" {
		metrics.WebhookFailures.Inc()
	}
	n.mark(ctx, webhookID, status, errMsg)
	_, err := n.Store.Queries.CreateWebhookDeliveryLog(ctx, sqlcdb.CreateWebhookDeliveryLogParams{
		WebhookID:   webhookID,
		Event:       event,
		Status:      status,
		StatusCode:  int32(statusCode),
		Error:       errMsg,
		RequestBody: body,
	})
	if err != nil {
		log.Printf("webhooks: log delivery %s: %v", webhookID, err)
	}
}

func (n *Notifier) mark(ctx context.Context, id uuid.UUID, status, errMsg string) {
	_, err := n.Store.Queries.MarkWebhookDelivery(ctx, sqlcdb.MarkWebhookDeliveryParams{
		ID:         id,
		LastStatus: status,
		LastError:  errMsg,
	})
	if err != nil {
		log.Printf("webhooks: mark %s: %v", id, err)
	}
}

// DeliverOne sends a test payload to a single webhook.
func (n *Notifier) DeliverOne(ctx context.Context, id uuid.UUID, event string, payload map[string]any) error {
	h, err := n.Store.Queries.GetWebhook(ctx, id)
	if err != nil {
		return err
	}
	env := Envelope{Event: event, Timestamp: time.Now().UTC(), Payload: payload}
	body, err := EncodeBody(h.Format, env)
	if err != nil {
		return err
	}
	n.deliver(ctx, h, body, event)
	return nil
}

// RetryDelivery re-sends a stored delivery body to its webhook.
func (n *Notifier) RetryDelivery(ctx context.Context, deliveryID uuid.UUID) error {
	d, err := n.Store.Queries.GetWebhookDeliveryLog(ctx, deliveryID)
	if err != nil {
		return err
	}
	h, err := n.Store.Queries.GetWebhook(ctx, d.WebhookID)
	if err != nil {
		return err
	}
	body := d.RequestBody
	if len(body) == 0 {
		return fmt.Errorf("delivery has empty body")
	}
	n.deliver(ctx, h, body, d.Event)
	return nil
}
