package tomo

import "time"

// Topology is the interface for network graph access.
// Defined here to avoid circular imports with the topology package.
type Topology interface {
	NumNodes() int
	NumLinks() int
	Links() []Link
	Nodes() []Node

	// Neighbors returns the node IDs adjacent to the given node.
	Neighbors(nodeID int) []int

	// ShortestPath returns the link indices on the shortest path from src to dst.
	// Returns nil, false if no path exists.
	ShortestPath(src, dst int) ([]int, bool)

	// AllPairsShortestPaths returns link index slices for all reachable (src, dst) pairs.
	AllPairsShortestPaths() []PathSpec
}

// Node represents a network node (router or AS).
type Node struct {
	ID        int
	Label     string
	Latitude  float64
	Longitude float64
}

// Link represents a directed network link between two nodes.
type Link struct {
	ID  int
	Src int
	Dst int
}

// Hop is a single hop in a traceroute-style measurement.
type Hop struct {
	IP  string
	RTT time.Duration
	TTL int
	// Anonymous is true if this hop did not respond (* * *).
	Anonymous bool
	// MPLS is true if this hop is inside an MPLS tunnel (RFC 4950 ICMP extension).
	MPLS bool
}

// PathSpec describes a measurement path through the network.
type PathSpec struct {
	Src      int   // source node ID
	Dst      int   // destination node ID
	LinkIDs  []int // ordered link indices traversed
}

// PathMeasurement is a single end-to-end observation.
type PathMeasurement struct {
	Src       string
	Dst       string
	Hops      []Hop
	RTTs      []time.Duration // multiple samples per path
	Timestamp time.Time
	Weight    float64 // measurement confidence (default 1.0)
}

// MinRTT returns the minimum RTT from all samples, which best
// approximates propagation delay by minimizing queueing noise.
func (m PathMeasurement) MinRTT() time.Duration {
	if len(m.RTTs) == 0 {
		return 0
	}
	min := m.RTTs[0]
	for _, rtt := range m.RTTs[1:] {
		if rtt < min {
			min = rtt
		}
	}
	return min
}
