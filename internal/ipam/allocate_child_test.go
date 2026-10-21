package ipam

import (
	"net/netip"
	"testing"
)

func TestFirstFreeChild(t *testing.T) {
	parent := netip.MustParsePrefix("10.0.0.0/24")
	occupied := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/25")}
	got, err := FirstFreeChild(parent, 25, occupied)
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "10.0.0.128/25" {
		t.Fatalf("got %s", got)
	}
}

func TestCoveringPrefix(t *testing.T) {
	a := netip.MustParsePrefix("10.0.0.0/25")
	b := netip.MustParsePrefix("10.0.0.128/25")
	got, err := CoveringPrefix(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "10.0.0.0/24" {
		t.Fatalf("got %s", got)
	}
}

func TestSplitHalves(t *testing.T) {
	left, right, err := SplitHalves(netip.MustParsePrefix("10.0.0.0/24"))
	if err != nil {
		t.Fatal(err)
	}
	if left.String() != "10.0.0.0/25" || right.String() != "10.0.0.128/25" {
		t.Fatalf("%s %s", left, right)
	}
}
