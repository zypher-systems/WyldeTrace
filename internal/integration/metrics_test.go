//go:build integration

package integration_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestMetricsEndpointUnauthenticated(t *testing.T) {
	h := openHarness(t)
	code, body := h.do(http.MethodGet, "/metrics", "", nil)
	if code != http.StatusOK {
		t.Fatalf("status %d: %s", code, body)
	}
	s := string(body)
	if !strings.Contains(s, "wyldetrace_http_requests_total") {
		t.Fatalf("expected http request metric in scrape output")
	}
}
