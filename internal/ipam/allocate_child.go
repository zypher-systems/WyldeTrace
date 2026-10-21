package ipam

import (
	"fmt"
	"net/netip"
)

// FirstFreeChild returns the first aligned child prefix of length childBits under
// parent that does not overlap any occupied prefix.
func FirstFreeChild(parent netip.Prefix, childBits int, occupied []netip.Prefix) (netip.Prefix, error) {
	children, err := ChildPrefixes(parent, childBits)
	if err != nil {
		return netip.Prefix{}, err
	}
	for _, c := range children {
		if prefixFree(c, occupied) {
			return c, nil
		}
	}
	return netip.Prefix{}, fmt.Errorf("no free /%d child under %s", childBits, parent)
}

// CoveringPrefix returns the smallest prefix that exactly covers a and b when they
// are equal-sized adjacent siblings; otherwise an error.
func CoveringPrefix(a, b netip.Prefix) (netip.Prefix, error) {
	a, b = a.Masked(), b.Masked()
	if a.Bits() != b.Bits() || a.Addr().BitLen() != b.Addr().BitLen() {
		return netip.Prefix{}, fmt.Errorf("merge requires equal prefix lengths")
	}
	if a.Bits() == 0 {
		return netip.Prefix{}, fmt.Errorf("cannot merge /0")
	}
	parentBits := a.Bits() - 1
	parent := netip.PrefixFrom(a.Addr(), parentBits).Masked()
	if !parent.Contains(a.Addr()) || !parent.Contains(b.Addr()) {
		return netip.Prefix{}, fmt.Errorf("prefixes are not siblings under a common parent")
	}
	kids, err := ChildPrefixes(parent, a.Bits())
	if err != nil || len(kids) != 2 {
		return netip.Prefix{}, fmt.Errorf("prefixes do not form a binary merge pair")
	}
	matchA := (kids[0] == a && kids[1] == b) || (kids[0] == b && kids[1] == a)
	if !matchA {
		return netip.Prefix{}, fmt.Errorf("prefixes are not an adjacent sibling pair")
	}
	return parent, nil
}

// SplitHalves returns the two equal children at parent.Bits()+1.
func SplitHalves(parent netip.Prefix) (left, right netip.Prefix, err error) {
	kids, err := ChildPrefixes(parent, parent.Bits()+1)
	if err != nil {
		return netip.Prefix{}, netip.Prefix{}, err
	}
	if len(kids) != 2 {
		return netip.Prefix{}, netip.Prefix{}, fmt.Errorf("expected 2 halves, got %d", len(kids))
	}
	return kids[0], kids[1], nil
}

func prefixFree(candidate netip.Prefix, occupied []netip.Prefix) bool {
	for _, o := range occupied {
		if Classify(candidate, o) != RelDisjoint {
			return false
		}
	}
	return true
}
