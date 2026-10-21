package webhook

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const (
	FormatJSON  = "json"
	FormatSlack = "slack"
	FormatTeams = "teams"
)

func NormalizeFormat(f string) string {
	switch strings.ToLower(strings.TrimSpace(f)) {
	case FormatSlack:
		return FormatSlack
	case FormatTeams:
		return FormatTeams
	default:
		return FormatJSON
	}
}

func ValidFormat(f string) bool {
	switch NormalizeFormat(f) {
	case FormatJSON, FormatSlack, FormatTeams:
		return true
	default:
		return false
	}
}

// EncodeBody renders the event envelope in the webhook's delivery format.
func EncodeBody(format string, env Envelope) ([]byte, error) {
	switch NormalizeFormat(format) {
	case FormatSlack:
		return json.Marshal(slackPayload(env))
	case FormatTeams:
		return json.Marshal(teamsPayload(env))
	default:
		return json.Marshal(env)
	}
}

func eventSummary(env Envelope) string {
	switch env.Event {
	case EventDiscoveryNewAlive:
		return fmt.Sprintf("Discovery: %v new alive host(s)", env.Payload["count"])
	case EventDiscoveryStale:
		return fmt.Sprintf("Discovery: %v stale host(s)", env.Payload["count"])
	case EventDiscoveryError:
		return fmt.Sprintf("Discovery error: %v", env.Payload["error"])
	case EventIPDeleted:
		return fmt.Sprintf("IP deleted: %v", env.Payload["address"])
	case EventIPClaimed:
		return fmt.Sprintf("IP claimed: %v", env.Payload["address"])
	case EventSubnetUtilHigh:
		return fmt.Sprintf("High utilization on %v (%.0f%%)", env.Payload["cidr"], asFloat(env.Payload["percent"]))
	case EventReportsSummary:
		return "WyldeTrace reports summary"
	default:
		return "WyldeTrace: " + env.Event
	}
}

func asFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int64:
		return float64(t)
	default:
		return 0
	}
}

func payloadLines(payload map[string]any) []string {
	if len(payload) == 0 {
		return nil
	}
	keys := make([]string, 0, len(payload))
	for k := range payload {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		v := payload[k]
		switch t := v.(type) {
		case []any:
			if len(t) > 8 {
				out = append(out, fmt.Sprintf("%s: %d items", k, len(t)))
			} else {
				out = append(out, fmt.Sprintf("%s: %v", k, t))
			}
		case map[string]any:
			out = append(out, fmt.Sprintf("%s: {…}", k))
		default:
			s := fmt.Sprint(v)
			if len(s) > 120 {
				s = s[:117] + "…"
			}
			out = append(out, fmt.Sprintf("%s: %s", k, s))
		}
	}
	if len(out) > 12 {
		out = out[:12]
	}
	return out
}

func slackPayload(env Envelope) map[string]any {
	text := eventSummary(env)
	lines := payloadLines(env.Payload)
	fields := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		fields = append(fields, map[string]any{
			"type": "mrkdwn",
			"text": "`" + line + "`",
		})
	}
	blocks := []map[string]any{
		{
			"type": "header",
			"text": map[string]any{"type": "plain_text", "text": text, "emoji": true},
		},
		{
			"type": "section",
			"text": map[string]any{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*Event:* `%s`\n*When:* %s", env.Event, env.Timestamp.Format("2006-01-02 15:04:05 UTC")),
			},
		},
	}
	if len(fields) > 0 {
		blocks = append(blocks, map[string]any{
			"type":   "section",
			"fields": fields,
		})
	}
	return map[string]any{
		"text":   text,
		"blocks": blocks,
	}
}

func teamsPayload(env Envelope) map[string]any {
	facts := []map[string]string{
		{"name": "Event", "value": env.Event},
		{"name": "When", "value": env.Timestamp.Format("2006-01-02 15:04:05 UTC")},
	}
	for _, line := range payloadLines(env.Payload) {
		name, value, ok := strings.Cut(line, ": ")
		if !ok {
			name, value = "Detail", line
		}
		facts = append(facts, map[string]string{"name": name, "value": value})
	}
	return map[string]any{
		"@type":      "MessageCard",
		"@context":   "https://schema.org/extensions",
		"summary":    eventSummary(env),
		"themeColor": "0EA5E9",
		"title":      eventSummary(env),
		"sections": []map[string]any{
			{
				"facts": facts,
				"text":  "WyldeTrace webhook notification",
			},
		},
	}
}
