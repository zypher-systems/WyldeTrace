package scanner

import (
	"net/netip"
	"testing"
)

func TestHostsFromPrefix24(t *testing.T) {
	p := netip.MustParsePrefix("10.0.0.0/24")
	hosts, err := hostsFromPrefix(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 254 {
		t.Fatalf("expected 254 hosts, got %d", len(hosts))
	}
	if hosts[0].String() != "10.0.0.1" {
		t.Fatalf("first host %s", hosts[0])
	}
}

func TestHostsTooLarge(t *testing.T) {
	p := netip.MustParsePrefix("10.0.0.0/16")
	_, err := hostsFromPrefix(p)
	if err == nil {
		t.Fatal("expected error for /16")
	}
}
