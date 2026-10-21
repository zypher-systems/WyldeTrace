package scanner

import (
	"context"
	"fmt"
	"net/netip"
	"runtime"
	"sync"

	"github.com/wyldetechnologies/wyldetrace/internal/ipam"
)

type ChildProbe struct {
	CIDR       string   `json:"cidr"`
	Alive      int      `json:"alive"`
	Sampled    int      `json:"sampled"`
	SampleIPs  []string `json:"sample_ips"`
	AliveIPs   []string `json:"alive_ips,omitempty"`
	LikelyUsed bool     `json:"likely_used"`
}

type DiscoverReport struct {
	ParentCIDR  string       `json:"parent_cidr"`
	ChildPrefix int          `json:"child_prefix"`
	Total       int          `json:"total"`
	LikelyUsed  int          `json:"likely_used"`
	Children    []ChildProbe `json:"children"`
}

type DiscoverOptions struct {
	ChildPrefix int
	SampleHosts int
	Workers     int
}

// DiscoverChildren probes sample hosts in each child prefix of a supernet.
func DiscoverChildren(ctx context.Context, parentCIDR string, opts DiscoverOptions) (*DiscoverReport, error) {
	parent, err := netip.ParsePrefix(parentCIDR)
	if err != nil {
		return nil, fmt.Errorf("invalid parent cidr: %w", err)
	}
	parent = parent.Masked()
	childBits := opts.ChildPrefix
	if childBits == 0 {
		if parent.Addr().Is4() {
			childBits = 24
		} else {
			childBits = parent.Bits() + 8
		}
	}
	sampleN := opts.SampleHosts
	if sampleN <= 0 {
		sampleN = 3
	}
	if sampleN > 8 {
		sampleN = 8
	}
	children, err := ipam.ChildPrefixes(parent, childBits)
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

	type job struct {
		idx int
		p   netip.Prefix
	}
	jobs := make(chan job)
	out := make([]ChildProbe, len(children))
	var wg sync.WaitGroup
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
				out[j.idx] = probeChild(ctx, j.p, sampleN)
			}
		}()
	}
	go func() {
		for i, p := range children {
			select {
			case <-ctx.Done():
				close(jobs)
				return
			case jobs <- job{idx: i, p: p}:
			}
		}
		close(jobs)
	}()
	wg.Wait()

	report := &DiscoverReport{
		ParentCIDR:  parent.String(),
		ChildPrefix: childBits,
		Total:       len(children),
		Children:    out,
	}
	for _, c := range out {
		if c.LikelyUsed {
			report.LikelyUsed++
		}
	}
	return report, ctx.Err()
}

func probeChild(ctx context.Context, prefix netip.Prefix, sampleN int) ChildProbe {
	samples := ipam.SampleAddrs(prefix, sampleN)
	cp := ChildProbe{
		CIDR:      prefix.String(),
		Sampled:   len(samples),
		SampleIPs: make([]string, 0, len(samples)),
		AliveIPs:  []string{},
	}
	for _, a := range samples {
		cp.SampleIPs = append(cp.SampleIPs, a.String())
		r := pingHost(ctx, a)
		if r.Alive {
			cp.Alive++
			cp.AliveIPs = append(cp.AliveIPs, a.String())
		}
	}
	cp.LikelyUsed = cp.Alive > 0
	return cp
}
