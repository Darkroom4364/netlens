package topology

import (
	"strings"

	"github.com/Darkroom4364/netlens/internal/tomo"
)

// ECMPResult holds ECMP detection results for a (src,dst) pair.
type ECMPResult struct {
	Src, Dst string
	NumPaths int        // number of distinct hop sequences
	HopSets  [][]string // each unique hop sequence
}

// DetectECMP groups measurements by (src,dst) and identifies pairs
// with multiple distinct hop sequences (evidence of ECMP load balancing).
func DetectECMP(measurements []tomo.PathMeasurement) []ECMPResult {
	type key struct{ src, dst string }

	groups := make(map[key][]tomo.PathMeasurement)
	for _, m := range measurements {
		k := key{m.Src, m.Dst}
		groups[k] = append(groups[k], m)
	}

	var results []ECMPResult
	for k, ms := range groups {
		seen := make(map[string][]string) // serialised sequence → hop list
		for _, m := range ms {
			seq := hopSequence(m)
			key := strings.Join(seq, "|")
			if _, ok := seen[key]; !ok {
				seen[key] = seq
			}
		}
		if len(seen) <= 1 {
			continue
		}
		var hopSets [][]string
		for _, s := range seen {
			hopSets = append(hopSets, s)
		}
		results = append(results, ECMPResult{
			Src:      k.src,
			Dst:      k.dst,
			NumPaths: len(hopSets),
			HopSets:  hopSets,
		})
	}
	return results
}

// DeduplicateECMP keeps only one measurement per (src,dst) pair,
// preferring the measurement with the fewest anonymous hops.
func DeduplicateECMP(measurements []tomo.PathMeasurement) []tomo.PathMeasurement {
	type key struct{ src, dst string }

	best := make(map[key]tomo.PathMeasurement)
	bestAnon := make(map[key]int)

	for _, m := range measurements {
		k := key{m.Src, m.Dst}
		anon := countAnonymous(m)
		if _, ok := best[k]; !ok || anon < bestAnon[k] {
			best[k] = m
			bestAnon[k] = anon
		}
	}

	out := make([]tomo.PathMeasurement, 0, len(best))
	for _, m := range best {
		out = append(out, m)
	}
	return out
}

// hopSequence extracts the ordered list of non-anonymous hop IPs from a measurement.
func hopSequence(m tomo.PathMeasurement) []string {
	var seq []string
	for _, h := range m.Hops {
		if !h.Anonymous {
			seq = append(seq, h.IP)
		}
	}
	return seq
}

// countAnonymous returns the number of anonymous hops in a measurement.
func countAnonymous(m tomo.PathMeasurement) int {
	n := 0
	for _, h := range m.Hops {
		if h.Anonymous {
			n++
		}
	}
	return n
}
