package topology

import (
	"fmt"

	"github.com/Darkroom4364/netlens/tomo"
)

// InferOpts controls the topology inference behavior.
type InferOpts struct {
	// MaxAnonymousFrac: discard paths with more than this fraction of anonymous hops (default 0.3).
	MaxAnonymousFrac float64
	// ASLevel: if true, use AS-level granularity instead of router-level (placeholder for future).
	ASLevel bool
	// AliasResolution: if true, run IP alias resolution (Kapar) to merge
	// multiple interfaces of the same router into a single node.
	AliasResolution bool
	// ECMPDetection: if true, deduplicate ECMP paths before building the graph,
	// keeping only the measurement with fewest anonymous hops per (src,dst) pair.
	ECMPDetection bool
}

// defaultMaxAnonymousFrac is used when MaxAnonymousFrac is zero.
const defaultMaxAnonymousFrac = 0.3

// InferFromMeasurements builds a network topology graph from traceroute path measurements.
//
// Each unique IP becomes a node. Consecutive non-anonymous hops define links.
// Anonymous hops are skipped (the surrounding hops are connected directly).
// MPLS hops are flagged but do not produce links (the underlying link is hidden).
// Paths exceeding the anonymous hop fraction threshold are discarded.
//
// Returns the inferred graph, a PathSpec per accepted measurement,
// the indices of accepted measurements from the input slice, and any error.
func InferFromMeasurements(measurements []tomo.PathMeasurement, opts InferOpts) (*Graph, []tomo.PathSpec, []int, error) {
	if len(measurements) == 0 {
		return nil, nil, nil, fmt.Errorf("topology: no measurements provided")
	}

	if opts.ECMPDetection {
		measurements = DeduplicateECMP(measurements)
	}

	maxAnonFrac := opts.MaxAnonymousFrac
	if maxAnonFrac == 0 {
		maxAnonFrac = defaultMaxAnonymousFrac
	}

	g := New()

	// ipToNodeID maps each unique IP string to a graph node ID.
	ipToNodeID := make(map[string]int)
	nextNodeID := 0

	// ensureNode returns the node ID for a given IP, creating one if needed.
	ensureNode := func(ip string) int {
		if id, ok := ipToNodeID[ip]; ok {
			return id
		}
		id := nextNodeID
		nextNodeID++
		ipToNodeID[ip] = id
		g.AddNode(tomo.Node{ID: id, Label: ip})
		return id
	}

	var pathSpecs []tomo.PathSpec
	var acceptedIdx []int

	for mi, m := range measurements {
		if len(m.Hops) == 0 {
			continue
		}

		// Check anonymous hop fraction.
		anonCount := 0
		for _, h := range m.Hops {
			if h.Anonymous {
				anonCount++
			}
		}
		anonFrac := float64(anonCount) / float64(len(m.Hops))
		if anonFrac > maxAnonFrac {
			continue
		}

		// Collect non-anonymous, non-MPLS hops that form visible links.
		// MPLS hops are skipped: the underlying physical link is hidden inside the tunnel.
		var visibleNodeIDs []int
		for _, h := range m.Hops {
			if h.Anonymous {
				continue
			}
			if h.MPLS {
				// MPLS tunnel hop — don't create a node/link for it.
				continue
			}
			nodeID := ensureNode(h.IP)
			visibleNodeIDs = append(visibleNodeIDs, nodeID)
		}

		// Build links between consecutive visible hops.
		var linkIDs []int
		for i := 0; i < len(visibleNodeIDs)-1; i++ {
			src := visibleNodeIDs[i]
			dst := visibleNodeIDs[i+1]
			if src == dst {
				continue // skip self-loops (duplicate IPs)
			}
			linkIdx := g.AddLink(src, dst)
			linkIDs = append(linkIDs, linkIdx)
		}

		// Determine src/dst node IDs for the PathSpec.
		if len(visibleNodeIDs) >= 2 {
			pathSpecs = append(pathSpecs, tomo.PathSpec{
				Src:     visibleNodeIDs[0],
				Dst:     visibleNodeIDs[len(visibleNodeIDs)-1],
				LinkIDs: linkIDs,
			})
			acceptedIdx = append(acceptedIdx, mi)
		}
	}

	if opts.AliasResolution {
		g = ResolveAliases(g)
	}

	return g, pathSpecs, acceptedIdx, nil
}
