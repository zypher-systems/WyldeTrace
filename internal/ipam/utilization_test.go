package ipam

import (
	"errors"
	"net/netip"
	"testing"
)

func TestIsSparsePrefix(t *testing.T) {
	cases := []struct {
		cidr   string
		sparse bool
	}{
		{"10.0.0.0/24", false},
		{"10.0.0.0/16", false},
		{"10.0.0.0/15", true},
		{"2001:db8::/120", false},
		{"2001:db8::/112", false},
		{"2001:db8::/111", true},
		{"2001:db8::/64", true},
	}
	for _, tc := range cases {
		p := netip.MustParsePrefix(tc.cidr)
		if got := IsSparsePrefix(p); got != tc.sparse {
			t.Fatalf("%s: sparse=%v want %v", tc.cidr, got, tc.sparse)
		}
	}
}

func TestSupportsAddressMap(t *testing.T) {
	if !SupportsAddressMap(netip.MustParsePrefix("10.0.0.0/24")) {
		t.Fatal("expected /24 map support")
	}
	if SupportsAddressMap(netip.MustParsePrefix("10.0.0.0/23")) {
		t.Fatal("/23 should not map")
	}
	if !SupportsAddressMap(netip.MustParsePrefix("2001:db8::/120")) {
		t.Fatal("expected /120 map support")
	}
	if SupportsAddressMap(netip.MustParsePrefix("2001:db8::/64")) {
		t.Fatal("/64 must not map")
	}
}

func TestTotalUsableSparse(t *testing.T) {
	_, err := TotalUsable(netip.MustParsePrefix("2001:db8::/64"))
	if !errors.Is(err, ErrSparsePrefix) {
		t.Fatalf("want ErrSparsePrefix, got %v", err)
	}
	n, err := TotalUsable(netip.MustParsePrefix("10.0.0.0/30"))
	if err != nil || n != 2 {
		t.Fatalf("dense /30: got %d %v", n, err)
	}
}

func TestComputeUtilizationSparse(t *testing.T) {
	p := netip.MustParsePrefix("2001:db8::/64")
	u := ComputeUtilization(p, []string{"2001:db8::1", "2001:db8::2"})
	if !u.Sparse {
		t.Fatal("expected sparse")
	}
	if u.Total != nil || u.Free != nil || u.Percent != nil {
		t.Fatalf("sparse util must omit total/free/percent: %+v", u)
	}
	if u.Used != 2 {
		t.Fatalf("used=%d", u.Used)
	}
	if u.NextIP == "" {
		t.Fatal("expected next_ip among known used")
	}
}

func TestComputeUtilizationDense(t *testing.T) {
	p := netip.MustParsePrefix("10.0.0.0/30")
	u := ComputeUtilization(p, []string{"10.0.0.1"})
	if u.Sparse || u.Total == nil || *u.Total != 2 || u.Used != 1 || u.Free == nil || *u.Free != 1 || u.NextIP != "10.0.0.2" {
		t.Fatalf("%+v", u)
	}
}

func TestNextFreeSparse(t *testing.T) {
	p := netip.MustParsePrefix("2001:db8::/64")
	used := map[netip.Addr]struct{}{
		netip.MustParseAddr("2001:db8::1"): {},
	}
	next, err := NextFreeSparse(p, used, netip.Addr{})
	if err != nil {
		t.Fatal(err)
	}
	if next.String() != "2001:db8::" {
		// :: is first usable for IPv6; if taken we'd get ::2 after ::1
		// with only ::1 used, :: is free
		t.Fatalf("got %s want 2001:db8::", next)
	}

	used[netip.MustParseAddr("2001:db8::")] = struct{}{}
	next, err = NextFreeSparse(p, used, netip.Addr{})
	if err != nil {
		t.Fatal(err)
	}
	if next.String() != "2001:db8::2" {
		t.Fatalf("got %s want 2001:db8::2", next)
	}

	// Hint cursor
	hint := netip.MustParseAddr("2001:db8::10")
	next, err = NextFreeSparse(p, used, hint)
	if err != nil {
		t.Fatal(err)
	}
	if next.String() != "2001:db8::10" {
		t.Fatalf("hint: got %s", next)
	}
}

func TestNextFreeSparseNoFullWalk(t *testing.T) {
	// Filling the first SparseNextFreeScanLimit addresses must not hang or OOM a /64.
	p := netip.MustParsePrefix("2001:db8::/64")
	used := make(map[netip.Addr]struct{}, SparseNextFreeScanLimit)
	cur := p.Addr()
	for i := 0; i < SparseNextFreeScanLimit; i++ {
		used[cur] = struct{}{}
		cur = cur.Next()
	}
	next, err := NextFreeSparse(p, used, netip.Addr{})
	if err != nil {
		t.Fatal(err)
	}
	// Should resume after max used via second strategy.
	want := cur.String()
	if next.String() != want {
		t.Fatalf("got %s want %s", next, want)
	}
}

func TestUsableHostsRejectsSparse(t *testing.T) {
	_, err := UsableHosts(netip.MustParsePrefix("2001:db8::/64"))
	if !errors.Is(err, ErrSparsePrefix) {
		t.Fatalf("got %v", err)
	}
}
