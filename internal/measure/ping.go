//go:build probing

package measure

import (
	"context"
	"fmt"
	"time"

	probing "github.com/prometheus-community/pro-bing"
	"golang.org/x/sync/errgroup"

	"github.com/Darkroom4364/netlens/tomo"
)

// ICMPProber performs traceroute-style probes using ICMP echo with
// incrementing TTL via pro-bing.
type ICMPProber struct {
	MaxHops int           // default 32
	Timeout time.Duration // per-hop timeout, default 2s
	Count   int           // packets per hop, default 3
}

// NewICMPProber returns an ICMPProber with sensible defaults.
func NewICMPProber() *ICMPProber {
	return &ICMPProber{
		MaxHops: 32,
		Timeout: 2 * time.Second,
		Count:   3,
	}
}

// Probe runs a traceroute-style probe to the target IP.
// Sends ICMP echo with incrementing TTL to discover hops.
func (p *ICMPProber) Probe(ctx context.Context, target string) (*tomo.PathMeasurement, error) {
	var hops []tomo.Hop
	var rtts []time.Duration
	var totalLoss float64

	for ttl := 1; ttl <= p.MaxHops; ttl++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		pinger, err := probing.NewPinger(target)
		if err != nil {
			return nil, fmt.Errorf("creating pinger for %s: %w", target, err)
		}
		pinger.SetPrivileged(true)
		pinger.TTL = ttl
		pinger.Count = p.Count
		pinger.Timeout = p.Timeout
		pinger.RecordRtts = true

		if err := pinger.Run(); err != nil {
			// Treat run errors at this TTL as an anonymous hop.
			hops = append(hops, tomo.Hop{TTL: ttl, Anonymous: true})
			continue
		}

		stats := pinger.Statistics()

		if stats.PacketsRecv == 0 {
			// No reply — anonymous hop (TTL expired or filtered).
			hops = append(hops, tomo.Hop{TTL: ttl, Anonymous: true})
			totalLoss += 1.0
			continue
		}

		hop := tomo.Hop{
			IP:  stats.Addr,
			RTT: stats.AvgRtt,
			TTL: ttl,
		}
		hops = append(hops, hop)
		rtts = append(rtts, stats.Rtts...)
		totalLoss += stats.PacketLoss / 100.0

		// Reached the destination — stop probing.
		if stats.Addr == target {
			break
		}
	}

	nHops := len(hops)
	avgLoss := 0.0
	if nHops > 0 {
		avgLoss = totalLoss / float64(nHops)
	}

	return &tomo.PathMeasurement{
		Src:       "local",
		Dst:       target,
		Hops:      hops,
		RTTs:      rtts,
		Loss:      avgLoss,
		Timestamp: time.Now(),
		Weight:    1.0,
	}, nil
}

// ProbeMultiple runs concurrent probes to multiple targets.
func (p *ICMPProber) ProbeMultiple(ctx context.Context, targets []string) ([]tomo.PathMeasurement, error) {
	results := make([]tomo.PathMeasurement, len(targets))

	g, gctx := errgroup.WithContext(ctx)
	for i, t := range targets {
		g.Go(func() error {
			m, err := p.Probe(gctx, t)
			if err != nil {
				return fmt.Errorf("probe %s: %w", t, err)
			}
			results[i] = *m
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return results, nil
}
