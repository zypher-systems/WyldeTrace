package ipam

import (
	"fmt"
	"net/netip"
)

const MaxMapAddresses = 256

// MapAddresses returns every address in prefix (including network/broadcast for IPv4)
// when the prefix contains at most MaxMapAddresses hosts. Used for IP status grids.
// Sparse / large prefixes (e.g. IPv6 /64) are rejected — register by address instead.
func MapAddresses(prefix netip.Prefix) ([]netip.Addr, error) {
	if !prefix.IsValid() {
		return nil, fmt.Errorf("invalid prefix")
	}
	prefix = prefix.Masked()
	if !SupportsAddressMap(prefix) {
		family := "IPv4 /24"
		if prefix.Addr().Is6() {
			family = "IPv6 /120"
		}
		return nil, fmt.Errorf("prefix too large for address map (max %d addresses; use %s or smaller — sparse prefixes are not enumerated)", MaxMapAddresses, family)
	}
	hostBits := prefix.Addr().BitLen() - prefix.Bits()
	total := 1 << hostBits
	out := make([]netip.Addr, 0, total)
	cur := prefix.Addr()
	for i := 0; i < total; i++ {
		if !prefix.Contains(cur) {
			break
		}
		out = append(out, cur)
		next := cur.Next()
		if next == cur {
			break
		}
		cur = next
	}
	return out, nil
}

// IsNetworkOrBroadcast reports whether addr is the IPv4 network or broadcast of prefix.
func IsNetworkOrBroadcast(prefix netip.Prefix, addr netip.Addr) bool {
	prefix = prefix.Masked()
	if !prefix.Addr().Is4() || prefix.Bits() >= 31 {
		return false
	}
	if addr == prefix.Addr() {
		return true
	}
	// last address in prefix
	hostBits := 32 - prefix.Bits()
	base := prefix.Addr().As4()
	v := uint32(base[0])<<24 | uint32(base[1])<<16 | uint32(base[2])<<8 | uint32(base[3])
	v |= (1 << hostBits) - 1
	bcast := netip.AddrFrom4([4]byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)})
	return addr == bcast
}
