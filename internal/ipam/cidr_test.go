package ipam

import (
	"net/netip"
	"testing"
)

func TestClassify(t *testing.T) {
	a := netip.MustParsePrefix("10.0.0.0/16")
	b := netip.MustParsePrefix("10.0.1.0/24")
	c := netip.MustParsePrefix("10.0.1.0/25")
	d := netip.MustParsePrefix("10.0.1.128/25")
	e := netip.MustParsePrefix("192.168.0.0/24")

	if Classify(a, b) != RelContains {
		t.Fatal("expected contains")
	}
	if Classify(b, a) != RelContained {
		t.Fatal("expected contained")
	}
	if Classify(c, d) != RelDisjoint {
		t.Fatal("expected disjoint siblings")
	}
	if Classify(b, e) != RelDisjoint {
		t.Fatal("expected disjoint")
	}
	if Classify(a, a) != RelEqual {
		t.Fatal("expected equal")
	}
}

func TestNextFree(t *testing.T) {
	p := netip.MustParsePrefix("10.0.0.0/30")
	used := map[netip.Addr]struct{}{
		netip.MustParseAddr("10.0.0.1"): {},
	}
	next, err := NextFree(p, used)
	if err != nil {
		t.Fatal(err)
	}
	if next.String() != "10.0.0.2" {
		t.Fatalf("got %s", next)
	}
}

func TestUtilization(t *testing.T) {
	p := netip.MustParsePrefix("10.0.0.0/30")
	u := ComputeUtilization(p, []string{"10.0.0.1"})
	if u.Sparse || u.Total == nil || *u.Total != 2 || u.Used != 1 || u.Free == nil || *u.Free != 1 || u.NextIP != "10.0.0.2" {
		t.Fatalf("%+v", u)
	}
}
