package webhook

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEncodeBodyFormats(t *testing.T) {
	env := Envelope{
		Event:     EventDiscoveryNewAlive,
		Timestamp: time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC),
		Payload:   map[string]any{"count": 2, "cidr": "10.0.0.0/24"},
	}
	raw, err := EncodeBody(FormatJSON, env)
	if err != nil {
		t.Fatal(err)
	}
	var envelope Envelope
	if err := json.Unmarshal(raw, &envelope); err != nil || envelope.Event != env.Event {
		t.Fatalf("json: %s", raw)
	}

	raw, err = EncodeBody(FormatSlack, env)
	if err != nil {
		t.Fatal(err)
	}
	var slack map[string]any
	if err := json.Unmarshal(raw, &slack); err != nil {
		t.Fatal(err)
	}
	if slack["text"] == nil || slack["blocks"] == nil {
		t.Fatalf("slack payload: %#v", slack)
	}

	raw, err = EncodeBody(FormatTeams, env)
	if err != nil {
		t.Fatal(err)
	}
	var teams map[string]any
	if err := json.Unmarshal(raw, &teams); err != nil {
		t.Fatal(err)
	}
	if teams["@type"] != "MessageCard" {
		t.Fatalf("teams payload: %#v", teams)
	}
}
