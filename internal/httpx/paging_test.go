package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseSort(t *testing.T) {
	allowed := map[string]bool{"cidr": true, "name": true}
	r := httptest.NewRequest(http.MethodGet, "/?sort=name&order=desc", nil)
	sq := ParseSort(r, allowed, "cidr", "asc")
	if sq.By != "name" || sq.Dir != "desc" {
		t.Fatalf("got %+v", sq)
	}

	r = httptest.NewRequest(http.MethodGet, "/?sort=evil&order=sideways", nil)
	sq = ParseSort(r, allowed, "cidr", "asc")
	if sq.By != "cidr" || sq.Dir != "asc" {
		t.Fatalf("expected defaults, got %+v", sq)
	}
}
