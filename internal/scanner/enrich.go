package scanner

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wyldetechnologies/wyldetrace/internal/oui"
)

// Common TCP ports probed during discovery enrichment.
var defaultProbePorts = []int{22, 80, 443, 445, 3389, 8080, 8443}

func EnrichAlive(ctx context.Context, results []Result, workers int) []Result {
	if workers <= 0 {
		workers = 16
	}
	type job struct {
		idx int
		ip  string
	}
	jobs := make(chan job)
	var wg sync.WaitGroup
	out := make([]Result, len(results))
	copy(out, results)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}
				enriched := enrichHost(ctx, out[j.idx])
				out[j.idx] = enriched
			}
		}()
	}

	go func() {
		for i, r := range results {
			if r.Alive {
				jobs <- job{idx: i, ip: r.IP}
			}
		}
		close(jobs)
	}()
	wg.Wait()
	return out
}

func enrichHost(ctx context.Context, r Result) Result {
	r.Hostname = lookupPTR(ctx, r.IP)
	r.OpenPorts = probeTCPPorts(ctx, r.IP, defaultProbePorts)
	r.MAC = lookupARP(r.IP)
	if len(r.OpenPorts) > 0 {
		r.Hints = inferHints(r.OpenPorts)
	}
	sug := oui.SuggestDeviceType(r.MAC, r.OpenPorts, r.Hints)
	r.Vendor = sug.Vendor
	r.DeviceHint = sug.DeviceTypeHint
	r.Hints = sug.Hints
	return r
}

func lookupPTR(ctx context.Context, ip string) string {
	resolver := net.DefaultResolver
	ctx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	defer cancel()
	names, err := resolver.LookupAddr(ctx, ip)
	if err != nil || len(names) == 0 {
		return ""
	}
	return strings.TrimSuffix(names[0], ".")
}

func probeTCPPorts(ctx context.Context, ip string, ports []int) []int {
	var open []int
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, p := range ports {
		wg.Add(1)
		go func(port int) {
			defer wg.Done()
			d := net.Dialer{Timeout: 400 * time.Millisecond}
			cctx, cancel := context.WithTimeout(ctx, 400*time.Millisecond)
			defer cancel()
			conn, err := d.DialContext(cctx, "tcp", net.JoinHostPort(ip, strconv.Itoa(port)))
			if err != nil {
				return
			}
			_ = conn.Close()
			mu.Lock()
			open = append(open, port)
			mu.Unlock()
		}(p)
	}
	wg.Wait()
	return open
}

func lookupARP(ip string) string {
	f, err := os.Open("/proc/net/arp")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	// skip header
	if sc.Scan() {
		_ = sc.Text()
	}
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 4 {
			continue
		}
		if fields[0] == ip {
			mac := fields[3]
			if mac == "00:00:00:00:00:00" {
				return ""
			}
			return strings.ToLower(mac)
		}
	}
	return ""
}

func inferHints(ports []int) []string {
	set := map[int]bool{}
	for _, p := range ports {
		set[p] = true
	}
	var hints []string
	if set[22] {
		hints = append(hints, "ssh")
	}
	if set[80] || set[443] || set[8080] || set[8443] {
		hints = append(hints, "web")
	}
	if set[445] {
		hints = append(hints, "smb")
	}
	if set[3389] {
		hints = append(hints, "rdp")
	}
	return hints
}

func FormatPorts(ports []int) string {
	if len(ports) == 0 {
		return ""
	}
	parts := make([]string, len(ports))
	for i, p := range ports {
		parts[i] = fmt.Sprintf("%d", p)
	}
	return strings.Join(parts, ",")
}
