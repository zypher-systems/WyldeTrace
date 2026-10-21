package powerdns

import (
	"net/netip"
	"testing"
)

func TestEnsureFQDNAndZone(t *testing.T) {
	if got := ensureFQDN("Host.Example.COM"); got != "host.example.com." {
		t.Fatalf("fqdn=%q", got)
	}
	if got := zoneFromName("app.lab.example.com"); got != "lab.example.com." {
		t.Fatalf("zone=%q", got)
	}
}

func TestPTRName(t *testing.T) {
	name, zone, err := ptrNameAndZone(netip.MustParseAddr("10.1.2.3"))
	if err != nil {
		t.Fatal(err)
	}
	if name != "3.2.1.10.in-addr.arpa." {
		t.Fatalf("name=%q", name)
	}
	if zone != "2.1.10.in-addr.arpa." {
		t.Fatalf("zone=%q", zone)
	}
}
