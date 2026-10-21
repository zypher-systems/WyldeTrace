//go:build integration

package integration_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/wyldetechnologies/wyldetrace/internal/auth"
)

func TestListEndpoints_ok(t *testing.T) {
	h := openHarness(t)
	_, adminTok := h.createUser(auth.RoleAdmin, uniqueName("list-admin")+"@test.local")
	_, roTok := h.createUser(auth.RoleReadOnly, uniqueName("list-ro")+"@test.local")

	type endpoint struct {
		name string
		path string
		tok  string
	}
	cases := []endpoint{
		{"locations", "/api/v1/locations", roTok},
		{"vrfs", "/api/v1/vrfs", roTok},
		{"vlans", "/api/v1/vlans", roTok},
		{"device_types", "/api/v1/device-types", roTok},
		{"custom_fields", "/api/v1/custom-fields", roTok},
		{"subnets", "/api/v1/subnets?page=1&page_size=10", roTok},
		{"subnets_tree", "/api/v1/subnets/tree", roTok},
		{"subnets_tags", "/api/v1/subnets/tags", roTok},
		{"ips", "/api/v1/ips?page=1&page_size=10", roTok},
		{"inbox", "/api/v1/inbox?page=1&page_size=10", roTok},
		{"ip_history", "/api/v1/ip-history?page=1&page_size=10", roTok},
		{"ip_requests", "/api/v1/ip-requests?page=1&page_size=10", roTok},
		{"notifications", "/api/v1/notifications", roTok},
		{"discovery_jobs", "/api/v1/discovery-jobs", roTok},
		{"reports_summary", "/api/v1/reports/summary", roTok},
		{"users", "/api/v1/users", adminTok},
		{"settings", "/api/v1/settings", adminTok},
		{"instance", "/api/v1/instance", adminTok},
		{"webhooks", "/api/v1/webhooks", adminTok},
		{"audit_logs", "/api/v1/audit-logs?page=1&page_size=10", adminTok},
		{"api_tokens", "/api/v1/api-tokens", adminTok},
		{"auth_me", "/api/v1/auth/me", roTok},
		{"auth_mfa_status", "/api/v1/auth/mfa", roTok},
		{"oidc_status", "/api/v1/auth/oidc/status", ""},
		{"healthz", "/api/v1/healthz", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, body := h.do(http.MethodGet, tc.path, tc.tok, nil)
			if code != http.StatusOK {
				t.Fatalf("GET %s status %d: %s", tc.path, code, body)
			}
			if len(body) == 0 {
				t.Fatalf("empty body for %s", tc.path)
			}
			// Smoke: JSON object or array.
			var anyJSON any
			if err := json.Unmarshal(body, &anyJSON); err != nil {
				t.Fatalf("not json: %v body=%s", err, body)
			}
		})
	}
}
