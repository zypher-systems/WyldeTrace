//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/wyldetechnologies/wyldetrace/internal/auth"
	"github.com/wyldetechnologies/wyldetrace/internal/db"
	sqlcdb "github.com/wyldetechnologies/wyldetrace/internal/db/sqlc"
)

func TestScope_membershipFailClosedAndScoped(t *testing.T) {
	h := openHarness(t)
	ctx := context.Background()

	locA := h.createLocation(uniqueName("site-a"))
	locB := h.createLocation(uniqueName("site-b"))
	cidrA := uniqueCIDR()
	cidrB := uniqueCIDR()
	subA := h.createSubnet(cidrA, "Scope A", locA)
	subB := h.createSubnet(cidrB, "Scope B", locB)

	addrA := strings.TrimSuffix(cidrA, ".0/24") + ".10"
	addrB := strings.TrimSuffix(cidrB, ".0/24") + ".10"
	markerA := "scope-marker-a-" + uuid.NewString()[:8]
	markerB := "scope-marker-b-" + uuid.NewString()[:8]

	if _, err := h.store.Queries.CreateIP(ctx, sqlcdb.CreateIPParams{
		Address: addrA, SubnetID: subA, VrfID: db.DefaultVRFID, Hostname: markerA, Status: "assigned",
		LocationID: db.UUIDToNull(locA), CustomFields: []byte(`{}`),
	}); err != nil {
		t.Fatalf("create ip a: %v", err)
	}
	if _, err := h.store.Queries.CreateIP(ctx, sqlcdb.CreateIPParams{
		Address: addrB, SubnetID: subB, VrfID: db.DefaultVRFID, Hostname: markerB, Status: "assigned",
		LocationID: db.UUIDToNull(locB), CustomFields: []byte(`{}`),
	}); err != nil {
		t.Fatalf("create ip b: %v", err)
	}
	if _, err := h.store.Queries.CreateIPHistory(ctx, sqlcdb.CreateIPHistoryParams{
		Address: addrA, SubnetID: db.UUIDToNull(subA), Hostname: markerA, Status: "assigned",
		LocationID: db.UUIDToNull(locA), Action: "create", ActorEmail: "admin@test", Details: []byte(`{}`),
	}); err != nil {
		t.Fatalf("history a: %v", err)
	}
	if _, err := h.store.Queries.CreateIPHistory(ctx, sqlcdb.CreateIPHistoryParams{
		Address: addrB, SubnetID: db.UUIDToNull(subB), Hostname: markerB, Status: "assigned",
		LocationID: db.UUIDToNull(locB), Action: "create", ActorEmail: "admin@test", Details: []byte(`{}`),
	}); err != nil {
		t.Fatalf("history b: %v", err)
	}
	jobA, err := h.store.Queries.CreateDiscoveryJob(ctx, sqlcdb.CreateDiscoveryJobParams{
		Name: uniqueName("job-a"), SubnetID: subA, IntervalMinutes: 60, Enabled: true, Enrich: false,
	})
	if err != nil {
		t.Fatalf("job a: %v", err)
	}
	jobB, err := h.store.Queries.CreateDiscoveryJob(ctx, sqlcdb.CreateDiscoveryJobParams{
		Name: uniqueName("job-b"), SubnetID: subB, IntervalMinutes: 60, Enabled: true, Enrich: false,
	})
	if err != nil {
		t.Fatalf("job b: %v", err)
	}
	t.Cleanup(func() {
		_ = h.store.Queries.DeleteDiscoveryJob(ctx, jobA.ID)
		_ = h.store.Queries.DeleteDiscoveryJob(ctx, jobB.ID)
	})

	_, adminTok := h.createUser(auth.RoleAdmin, uniqueName("admin")+"@test.local")
	blockedID, blockedTok := h.createUser(auth.RoleEditor, uniqueName("blocked")+"@test.local")
	h.setUserLocations(blockedID, nil)
	scopedID, scopedTok := h.createUser(auth.RoleEditor, uniqueName("scoped")+"@test.local")
	h.setUserLocations(scopedID, []uuid.UUID{locA})

	type searchResp struct {
		IPs []struct {
			Hostname string `json:"hostname"`
		} `json:"ips"`
	}

	tests := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "fail_closed_search_empty",
			fn: func(t *testing.T) {
				code, body := h.do(http.MethodGet, "/api/v1/search?q="+markerA, blockedTok, nil)
				if code != http.StatusOK {
					t.Fatalf("status %d: %s", code, body)
				}
				var resp searchResp
				if err := json.Unmarshal(body, &resp); err != nil {
					t.Fatal(err)
				}
				if len(resp.IPs) != 0 {
					t.Fatalf("fail-closed user should see no IPs, got %d", len(resp.IPs))
				}
			},
		},
		{
			name: "scoped_search_sees_only_a",
			fn: func(t *testing.T) {
				code, body := h.do(http.MethodGet, "/api/v1/search?q="+markerA, scopedTok, nil)
				if code != http.StatusOK {
					t.Fatalf("status %d: %s", code, body)
				}
				var resp searchResp
				if err := json.Unmarshal(body, &resp); err != nil {
					t.Fatal(err)
				}
				if len(resp.IPs) < 1 {
					t.Fatalf("expected in-scope IP, body=%s", body)
				}
				code, body = h.do(http.MethodGet, "/api/v1/search?q="+markerB, scopedTok, nil)
				if code != http.StatusOK {
					t.Fatalf("status %d: %s", code, body)
				}
				if err := json.Unmarshal(body, &resp); err != nil {
					t.Fatal(err)
				}
				if len(resp.IPs) != 0 {
					t.Fatalf("scoped user must not see site B IP")
				}
			},
		},
		{
			name: "unscoped_admin_sees_both",
			fn: func(t *testing.T) {
				code, body := h.do(http.MethodGet, "/api/v1/search?q="+markerB, adminTok, nil)
				if code != http.StatusOK {
					t.Fatalf("status %d: %s", code, body)
				}
				var resp searchResp
				if err := json.Unmarshal(body, &resp); err != nil {
					t.Fatal(err)
				}
				if len(resp.IPs) < 1 {
					t.Fatalf("admin should see site B, body=%s", body)
				}
			},
		},
		{
			name: "history_scoped",
			fn: func(t *testing.T) {
				code, body := h.do(http.MethodGet, "/api/v1/ip-history?page=1&page_size=50&address="+addrA, scopedTok, nil)
				if code != http.StatusOK {
					t.Fatalf("history a status %d: %s", code, body)
				}
				if !strings.Contains(string(body), markerA) {
					t.Fatalf("expected history for A: %s", body)
				}
				code, body = h.do(http.MethodGet, "/api/v1/ip-history?page=1&page_size=50&address="+addrB, scopedTok, nil)
				if code != http.StatusOK {
					t.Fatalf("history b status %d: %s", code, body)
				}
				if strings.Contains(string(body), markerB) {
					t.Fatalf("scoped must not see history B: %s", body)
				}
			},
		},
		{
			name: "jobs_scoped",
			fn: func(t *testing.T) {
				code, body := h.do(http.MethodGet, "/api/v1/discovery-jobs", scopedTok, nil)
				if code != http.StatusOK {
					t.Fatalf("jobs status %d: %s", code, body)
				}
				var jobs []struct {
					ID uuid.UUID `json:"id"`
				}
				if err := json.Unmarshal(body, &jobs); err != nil {
					t.Fatal(err)
				}
				seenA, seenB := false, false
				for _, j := range jobs {
					if j.ID == jobA.ID {
						seenA = true
					}
					if j.ID == jobB.ID {
						seenB = true
					}
				}
				if !seenA || seenB {
					t.Fatalf("scoped jobs: seenA=%v seenB=%v", seenA, seenB)
				}
			},
		},
		{
			name: "sweep_out_of_scope_forbidden",
			fn: func(t *testing.T) {
				code, body := h.do(http.MethodPost, "/api/v1/scanner/sweep", scopedTok, map[string]any{
					"cidr": cidrB, "subnet_id": subB, "enrich": false, "workers": 1,
				})
				if code != http.StatusForbidden {
					t.Fatalf("expected 403 for out-of-scope sweep, got %d: %s", code, body)
				}
			},
		},
		{
			name: "import_out_of_scope_skipped",
			fn: func(t *testing.T) {
				csvBody := fmt.Sprintf("cidr,name,location_id\n198.18.%d.0/24,OutOfScopeImport,%s\n",
					int(uuid.New().ID()%200), locB)
				req, err := http.NewRequest(http.MethodPost, h.srv.URL+"/api/v1/import/subnets.csv", bytes.NewReader([]byte(csvBody)))
				if err != nil {
					t.Fatal(err)
				}
				req.Header.Set("Authorization", "Bearer "+scopedTok)
				req.Header.Set("Content-Type", "text/csv")
				res, err := http.DefaultClient.Do(req)
				if err != nil {
					t.Fatal(err)
				}
				defer res.Body.Close()
				var summary struct {
					Skipped int      `json:"skipped"`
					Errors  []string `json:"errors"`
				}
				if err := json.NewDecoder(res.Body).Decode(&summary); err != nil {
					t.Fatal(err)
				}
				if res.StatusCode != http.StatusOK {
					t.Fatalf("import status %d summary=%+v", res.StatusCode, summary)
				}
				if summary.Skipped < 1 {
					t.Fatalf("expected out-of-scope import skip, got %+v", summary)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, tc.fn)
	}
}
