package webhook

import "testing"

func TestSubscribes(t *testing.T) {
	if !Subscribes("discovery.new_alive,discovery.stale", EventDiscoveryNewAlive) {
		t.Fatal("expected subscribe")
	}
	if Subscribes("discovery.stale", EventDiscoveryNewAlive) {
		t.Fatal("expected no subscribe")
	}
	if !Subscribes("*", EventDiscoveryError) {
		t.Fatal("star should match")
	}
}
