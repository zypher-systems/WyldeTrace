package scanner

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

const MaxHosts = 256 // /24 cap for MVP

type Result struct {
	IP         string   `json:"ip"`
	Alive      bool     `json:"alive"`
	RTTMs      float64  `json:"rtt_ms,omitempty"`
	Error      string   `json:"error,omitempty"`
	Hostname   string   `json:"hostname,omitempty"`
	MAC        string   `json:"mac,omitempty"`
	Vendor     string   `json:"vendor,omitempty"`
	DeviceHint string   `json:"device_hint,omitempty"`
	OpenPorts  []int    `json:"open_ports,omitempty"`
	Hints      []string `json:"hints,omitempty"`
}

type SweepOptions struct {
	Workers int
	Enrich  bool
}

type SweepReport struct {
	CIDR    string   `json:"cidr"`
	Total   int      `json:"total"`
	Alive   int      `json:"alive"`
	Results []Result `json:"results"`
}

func Sweep(ctx context.Context, cidr string, opts SweepOptions) (*SweepReport, error) {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return nil, fmt.Errorf("invalid cidr: %w", err)
	}
	hosts, err := hostsFromPrefix(prefix)
	if err != nil {
		return nil, err
	}
	workers := opts.Workers
	if workers <= 0 {
		workers = runtime.NumCPU() * 4
	}
	if workers > 64 {
		workers = 64
	}

	jobs := make(chan netip.Addr)
	out := make(chan Result, len(hosts))
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ip := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}
				out <- pingHost(ctx, ip)
			}
		}()
	}

	go func() {
		for _, h := range hosts {
			select {
			case <-ctx.Done():
				close(jobs)
				return
			case jobs <- h:
			}
		}
		close(jobs)
	}()

	go func() {
		wg.Wait()
		close(out)
	}()

	report := &SweepReport{CIDR: prefix.String(), Total: len(hosts), Results: make([]Result, 0, len(hosts))}
	for r := range out {
		if r.Alive {
			report.Alive++
		}
		report.Results = append(report.Results, r)
	}
	if opts.Enrich {
		report.Results = EnrichAlive(ctx, report.Results, workers)
	}
	return report, ctx.Err()
}

func hostsFromPrefix(prefix netip.Prefix) ([]netip.Addr, error) {
	prefix = prefix.Masked()
	hostBits := prefix.Addr().BitLen() - prefix.Bits()
	if hostBits < 0 {
		return nil, fmt.Errorf("invalid prefix")
	}
	// Hard cap: never enumerate sparse / large prefixes (e.g. IPv6 /64).
	if hostBits > 8 {
		family := "IPv4 /24"
		if prefix.Addr().Is6() {
			family = "IPv6 /120"
		}
		return nil, fmt.Errorf("subnet too large for dense sweep: max %d hosts (use %s or smaller; register observed hosts individually)", MaxHosts, family)
	}
	var hosts []netip.Addr
	addr := prefix.Addr()
	for {
		if !prefix.Contains(addr) {
			break
		}
		ones := prefix.Bits()
		isIPv4 := addr.Is4()
		if isIPv4 && ones < 31 {
			if len(hosts) == 0 && addr == prefix.Addr() {
				addr = addr.Next()
				continue
			}
		}
		hosts = append(hosts, addr)
		if len(hosts) > MaxHosts {
			return nil, fmt.Errorf("subnet too large: max %d hosts (use /24 or smaller)", MaxHosts)
		}
		next := addr.Next()
		if next == addr {
			break
		}
		addr = next
	}

	if prefix.Addr().Is4() && prefix.Bits() < 31 && len(hosts) > 0 {
		hosts = hosts[:len(hosts)-1]
	}
	return hosts, nil
}

func pingHost(ctx context.Context, ip netip.Addr) Result {
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	args := pingArgs(ip.String())
	cmd := exec.CommandContext(ctx, "ping", args...)
	err := cmd.Run()
	rtt := float64(time.Since(start).Microseconds()) / 1000.0
	if err != nil {
		return Result{IP: ip.String(), Alive: false}
	}
	return Result{IP: ip.String(), Alive: true, RTTMs: rtt}
}

func pingArgs(ip string) []string {
	if runtime.GOOS == "windows" {
		return []string{"-n", "1", "-w", "1000", ip}
	}
	return []string{"-c", "1", "-W", "1", ip}
}

func DialProbe(ctx context.Context, ip string, port string) bool {
	d := net.Dialer{Timeout: time.Second}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(ip, port))
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func NormalizeCIDR(s string) (string, error) {
	p, err := netip.ParsePrefix(strings.TrimSpace(s))
	if err != nil {
		return "", err
	}
	return p.Masked().String(), nil
}
