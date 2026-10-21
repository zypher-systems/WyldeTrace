package ipam

import (
	"errors"
	"fmt"
	"net/netip"
)

// Dense vs sparse thresholds:
//   - IPv4 prefixes longer than /16 (bits >= 16) are dense and may be enumerated.
//   - IPv6 prefixes longer than /112 (bits >= 112) are dense; a /64 is always sparse.
// Address maps / full ICMP sweeps additionally require hostBits <= MaxMapHostBits (256 addrs).
const (
	DenseIPv4MinBits       = 16
	DenseIPv6MinBits       = 112
	MaxMapHostBits         = 8 // /24 IPv4, /120 IPv6
	SparseNextFreeScanLimit = 4096
)

// ErrSparsePrefix indicates the prefix is too large to treat as a dense host pool.
var ErrSparsePrefix = errors.New("sparse prefix: host space is not enumerable")

type Utilization struct {
	CIDR      string   `json:"cidr"`
	Total     *int     `json:"total,omitempty"`
	Used      int      `json:"used"`
	Free      *int     `json:"free,omitempty"`
	Percent   *float64 `json:"percent,omitempty"`
	Sparse    bool     `json:"sparse,omitempty"`
	NextIP    string   `json:"next_ip,omitempty"`
	NextIPErr string   `json:"next_ip_error,omitempty"`
}

// IsSparsePrefix reports whether the prefix is too large for dense host enumeration.
// Sparse prefixes never walk the full host space (e.g. IPv6 /64).
func IsSparsePrefix(prefix netip.Prefix) bool {
	if !prefix.IsValid() {
		return true
	}
	prefix = prefix.Masked()
	bits := prefix.Bits()
	if prefix.Addr().Is4() {
		return bits < DenseIPv4MinBits
	}
	return bits < DenseIPv6MinBits
}

// SupportsAddressMap reports whether a dense occupancy grid can be rendered
// (at most 256 addresses: IPv4 /24+, IPv6 /120+).
func SupportsAddressMap(prefix netip.Prefix) bool {
	if !prefix.IsValid() {
		return false
	}
	prefix = prefix.Masked()
	hostBits := prefix.Addr().BitLen() - prefix.Bits()
	return hostBits >= 0 && hostBits <= MaxMapHostBits
}

// SupportsDenseSweep reports whether a full ICMP sweep of every usable host is allowed.
func SupportsDenseSweep(prefix netip.Prefix) bool {
	return SupportsAddressMap(prefix)
}

func UsableHosts(prefix netip.Prefix) ([]netip.Addr, error) {
	if !prefix.IsValid() {
		return nil, fmt.Errorf("invalid prefix")
	}
	prefix = prefix.Masked()
	if IsSparsePrefix(prefix) {
		return nil, fmt.Errorf("%w (%s)", ErrSparsePrefix, prefix)
	}

	bits := prefix.Bits()
	addr := prefix.Addr()

	var hosts []netip.Addr
	cur := addr
	first := true
	for prefix.Contains(cur) {
		skip := false
		if addr.Is4() && bits < 31 {
			if first {
				// network address
				skip = true
			}
		}
		first = false
		next := cur.Next()
		isLast := !prefix.Contains(next) || next == cur
		if addr.Is4() && bits < 31 && isLast {
			// broadcast
			break
		}
		if !skip {
			hosts = append(hosts, cur)
		}
		if next == cur {
			break
		}
		cur = next
	}
	return hosts, nil
}

// TotalUsable returns the countable usable host total for dense prefixes.
// For sparse prefixes it returns ErrSparsePrefix (callers should omit total/percent).
func TotalUsable(prefix netip.Prefix) (int, error) {
	if !prefix.IsValid() {
		return 0, fmt.Errorf("invalid prefix")
	}
	prefix = prefix.Masked()
	if IsSparsePrefix(prefix) {
		return 0, ErrSparsePrefix
	}
	bits := prefix.Bits()
	addr := prefix.Addr()
	hostBits := addr.BitLen() - bits
	if hostBits < 0 {
		return 0, fmt.Errorf("invalid prefix")
	}
	total := 1 << hostBits
	if addr.Is4() && bits < 31 {
		total -= 2
	}
	if total < 0 {
		total = 0
	}
	return total, nil
}

func ComputeUtilization(prefix netip.Prefix, usedAddrs []string) Utilization {
	usedSet := UsedSet(usedAddrs)
	u := Utilization{
		CIDR:   prefix.String(),
		Used:   len(usedSet),
		Sparse: IsSparsePrefix(prefix),
	}
	if !u.Sparse {
		total, err := TotalUsable(prefix)
		if err != nil {
			u.NextIPErr = err.Error()
		} else {
			free := total - u.Used
			if free < 0 {
				free = 0
			}
			pct := 0.0
			if total > 0 {
				pct = float64(u.Used) / float64(total) * 100
			}
			u.Total = &total
			u.Free = &free
			u.Percent = &pct
		}
	}
	next, nerr := NextFree(prefix, usedSet)
	if nerr != nil {
		u.NextIPErr = nerr.Error()
	} else {
		u.NextIP = next.String()
	}
	return u
}

// NextFree returns the next unused address. Dense prefixes walk usable hosts;
// sparse prefixes use NextFreeSparse (bounded scan, never full /64 walk).
func NextFree(prefix netip.Prefix, used map[netip.Addr]struct{}) (netip.Addr, error) {
	if IsSparsePrefix(prefix) {
		return NextFreeSparse(prefix, used, netip.Addr{})
	}
	hosts, err := UsableHosts(prefix)
	if err != nil {
		return netip.Addr{}, err
	}
	for _, h := range hosts {
		if _, ok := used[h]; !ok {
			return h, nil
		}
	}
	return netip.Addr{}, fmt.Errorf("no free addresses in %s", prefix)
}

// NextFreeSparse finds a free address without enumerating the full host space.
// When startHint is valid and inside the prefix, scanning begins there; otherwise
// it starts at the first usable address. If the forward scan hits SparseNextFreeScanLimit
// without success, it retries once from max(used)+1.
func NextFreeSparse(prefix netip.Prefix, used map[netip.Addr]struct{}, startHint netip.Addr) (netip.Addr, error) {
	if !prefix.IsValid() {
		return netip.Addr{}, fmt.Errorf("invalid prefix")
	}
	prefix = prefix.Masked()
	start := firstUsable(prefix)
	if startHint.IsValid() && prefix.Contains(startHint) {
		start = startHint
		if IsNetworkOrBroadcast(prefix, start) {
			start = start.Next()
		}
	}
	if addr, ok := scanFree(prefix, used, start, SparseNextFreeScanLimit); ok {
		return addr, nil
	}
	// Second chance: resume after the highest known used address.
	if maxUsed, found := maxUsedInPrefix(prefix, used); found {
		resume := maxUsed.Next()
		if resume != maxUsed && prefix.Contains(resume) && resume != start {
			if addr, ok := scanFree(prefix, used, resume, SparseNextFreeScanLimit); ok {
				return addr, nil
			}
		}
	}
	return netip.Addr{}, fmt.Errorf("no free address found in %s within scan limit (register by address or tighten the pool)", prefix)
}

func firstUsable(prefix netip.Prefix) netip.Addr {
	a := prefix.Masked().Addr()
	if prefix.Addr().Is4() && prefix.Bits() < 31 {
		return a.Next()
	}
	return a
}

func scanFree(prefix netip.Prefix, used map[netip.Addr]struct{}, start netip.Addr, limit int) (netip.Addr, bool) {
	if !start.IsValid() || !prefix.Contains(start) {
		return netip.Addr{}, false
	}
	cur := start
	for i := 0; i < limit; i++ {
		if !prefix.Contains(cur) {
			return netip.Addr{}, false
		}
		if IsNetworkOrBroadcast(prefix, cur) {
			next := cur.Next()
			if next == cur {
				return netip.Addr{}, false
			}
			cur = next
			continue
		}
		if _, taken := used[cur]; !taken {
			return cur, true
		}
		next := cur.Next()
		if next == cur {
			return netip.Addr{}, false
		}
		cur = next
	}
	return netip.Addr{}, false
}

func maxUsedInPrefix(prefix netip.Prefix, used map[netip.Addr]struct{}) (netip.Addr, bool) {
	var max netip.Addr
	found := false
	for a := range used {
		if !prefix.Contains(a) {
			continue
		}
		if !found || a.Compare(max) > 0 {
			max = a
			found = true
		}
	}
	return max, found
}

func UsedSet(addrs []string) map[netip.Addr]struct{} {
	m := make(map[netip.Addr]struct{}, len(addrs))
	for _, s := range addrs {
		if a, err := netip.ParseAddr(s); err == nil {
			m[a] = struct{}{}
		}
	}
	return m
}
