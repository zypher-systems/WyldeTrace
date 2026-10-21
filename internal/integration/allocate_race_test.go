//go:build integration

package integration_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/wyldetechnologies/wyldetrace/internal/auth"
)

func TestAllocateNextIP_concurrentRaceRetry(t *testing.T) {
	h := openHarness(t)

	loc := h.createLocation(uniqueName("alloc"))
	id := uuid.New()
	cidr := fmt.Sprintf("10.%d.%d.0/29", id[0], id[1])
	subnetID := h.createSubnet(cidr, "Alloc Race", loc)

	edID, tok := h.createUser(auth.RoleEditor, uniqueName("alloc-ed")+"@test.local")
	h.setUserLocations(edID, []uuid.UUID{loc})

	const n = 4
	var wg sync.WaitGroup
	addrs := make(chan string, n)
	var okCount atomic.Int32

	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			code, body := h.do(http.MethodPost, "/api/v1/subnets/"+subnetID.String()+"/ips/next", tok, map[string]any{
				"status": "assigned", "hostname": "race-" + uuid.NewString()[:6],
			})
			if code != http.StatusCreated {
				t.Logf("allocate status %d: %s", code, body)
				return
			}
			var resp struct {
				Address string `json:"address"`
			}
			if err := json.Unmarshal(body, &resp); err != nil {
				t.Errorf("decode: %v body=%s", err, body)
				return
			}
			addrs <- resp.Address
			okCount.Add(1)
		}()
	}
	wg.Wait()
	close(addrs)

	seen := map[string]bool{}
	for a := range addrs {
		if seen[a] {
			t.Fatalf("duplicate address allocated under race: %s", a)
		}
		seen[a] = true
	}
	if okCount.Load() < 2 {
		t.Fatalf("expected concurrent allocates to succeed via unique-violation retry, got ok=%d", okCount.Load())
	}
}
