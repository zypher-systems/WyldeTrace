package ipam

import (
	"fmt"
	"net/netip"
)

type Relation int

const (
	RelDisjoint Relation = iota
	RelEqual
	RelContains   // a contains b
	RelContained  // a contained by b
	RelOverlap    // partial overlap (illegal for sibling allocations)
)

func ParsePrefix(cidr string) (netip.Prefix, error) {
	p, err := netip.ParsePrefix(cidr)
	if err != nil {
		return netip.Prefix{}, err
	}
	return p.Masked(), nil
}

func Classify(a, b netip.Prefix) Relation {
	if a == b {
		return RelEqual
	}
	if a.Overlaps(b) {
		if a.Bits() < b.Bits() && a.Contains(b.Addr()) {
			return RelContains
		}
		if b.Bits() < a.Bits() && b.Contains(a.Addr()) {
			return RelContained
		}
		return RelOverlap
	}
	return RelDisjoint
}

func ContainsPrefix(parent, child netip.Prefix) bool {
	rel := Classify(parent, child)
	return rel == RelContains || rel == RelEqual
}

const MaxChildPrefixes = 256

// ChildPrefixes enumerates masked child prefixes inside parent at childBits.
func ChildPrefixes(parent netip.Prefix, childBits int) ([]netip.Prefix, error) {
	parent = parent.Masked()
	if childBits <= parent.Bits() {
		return nil, fmt.Errorf("child prefix length must be longer than parent (/%d)", parent.Bits())
	}
	if parent.Addr().Is4() && childBits > 32 {
		return nil, fmt.Errorf("invalid IPv4 child prefix length")
	}
	if parent.Addr().Is6() && childBits > 128 {
		return nil, fmt.Errorf("invalid IPv6 child prefix length")
	}
	delta := childBits - parent.Bits()
	if delta > 16 {
		return nil, fmt.Errorf("too many child prefixes (max span 16 bits)")
	}
	count := 1 << delta
	if count > MaxChildPrefixes {
		return nil, fmt.Errorf("too many child prefixes: %d (max %d); use a tighter supernet or longer child mask", count, MaxChildPrefixes)
	}

	out := make([]netip.Prefix, 0, count)
	cur, err := netip.ParsePrefix(fmt.Sprintf("%s/%d", parent.Addr().String(), childBits))
	if err != nil {
		return nil, err
	}
	cur = cur.Masked()
	for i := 0; i < count; i++ {
		if !parent.Contains(cur.Addr()) {
			break
		}
		out = append(out, cur)
		nextAddr, ok := addPrefixStep(cur)
		if !ok {
			break
		}
		cur = netip.PrefixFrom(nextAddr, childBits).Masked()
	}
	return out, nil
}

func addPrefixStep(p netip.Prefix) (netip.Addr, bool) {
	addr := p.Addr()
	bits := p.Bits()
	// Advance by 2^(host bits) addresses.
	hostBits := addr.BitLen() - bits
	if hostBits <= 0 {
		return netip.Addr{}, false
	}
	step := uint64(1) << uint(hostBits)
	if addr.Is4() {
		b := addr.As4()
		v := uint64(b[0])<<24 | uint64(b[1])<<16 | uint64(b[2])<<8 | uint64(b[3])
		v += step
		if v > 0xffffffff {
			return netip.Addr{}, false
		}
		return netip.AddrFrom4([4]byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}), true
	}
	// IPv6: only support stepping within last 64 bits for MVP.
	b := addr.As16()
	var hi, lo uint64
	for i := 0; i < 8; i++ {
		hi = hi<<8 | uint64(b[i])
		lo = lo<<8 | uint64(b[8+i])
	}
	nlo := lo + step
	if nlo < lo {
		hi++
	}
	var out [16]byte
	for i := 7; i >= 0; i-- {
		out[i] = byte(hi)
		hi >>= 8
		out[8+i] = byte(nlo)
		nlo >>= 8
	}
	return netip.AddrFrom16(out), true
}

// SampleAddrs returns up to n usable probe addresses inside prefix.
func SampleAddrs(prefix netip.Prefix, n int) []netip.Addr {
	if n <= 0 {
		return nil
	}
	hosts, err := UsableHosts(prefix)
	if err != nil || len(hosts) == 0 {
		// fallback: network+1 style for large nets
		a := prefix.Masked().Addr()
		if prefix.Addr().Is4() && prefix.Bits() < 31 {
			a = a.Next()
		}
		out := []netip.Addr{a}
		for i := 1; i < n; i++ {
			next := out[len(out)-1].Next()
			if !prefix.Contains(next) || next == out[len(out)-1] {
				break
			}
			out = append(out, next)
		}
		return out
	}
	if len(hosts) <= n {
		return hosts
	}
	out := make([]netip.Addr, 0, n)
	out = append(out, hosts[0])
	if n >= 2 {
		out = append(out, hosts[1])
	}
	if n >= 3 {
		out = append(out, hosts[len(hosts)/2])
	}
	if n >= 4 {
		out = append(out, hosts[len(hosts)-1])
	}
	for len(out) < n && len(out) < len(hosts) {
		out = append(out, hosts[len(out)])
	}
	return out[:min(n, len(out))]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
