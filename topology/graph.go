package topology

import (
	"math"
	"sync"

	"github.com/Darkroom4364/netlens/tomo"
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/path"
	"gonum.org/v1/gonum/graph/simple"
)

// Graph is a network topology backed by gonum's undirected graph.
// It implements tomo.Topology.
type Graph struct {
	mu    sync.RWMutex
	g     *simple.UndirectedGraph
	nodes []tomo.Node
	links []tomo.Link

	// nodeMap maps node ID → index in nodes slice
	nodeMap map[int]int
	// linkMap maps "src-dst" (sorted) → link index
	linkMap map[[2]int]int
}

// New creates an empty Graph.
func New() *Graph {
	return &Graph{
		g:       simple.NewUndirectedGraph(),
		nodeMap: make(map[int]int),
		linkMap: make(map[[2]int]int),
	}
}

// AddNode adds a node to the graph.
func (g *Graph) AddNode(n tomo.Node) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.nodeMap[n.ID] = len(g.nodes)
	g.nodes = append(g.nodes, n)
	g.g.AddNode(simple.Node(n.ID))
}

// AddLink adds a bidirectional link between two nodes.
// Self-loops (src == dst) are rejected and return -1.
func (g *Graph) AddLink(src, dst int) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	if src == dst {
		return -1
	}
	key := linkKey(src, dst)
	if idx, ok := g.linkMap[key]; ok {
		return idx
	}

	idx := len(g.links)
	g.links = append(g.links, tomo.Link{
		ID:  idx,
		Src: src,
		Dst: dst,
	})
	g.linkMap[key] = idx
	g.g.SetEdge(simple.Edge{F: simple.Node(src), T: simple.Node(dst)})
	return idx
}

func linkKey(a, b int) [2]int {
	if a > b {
		a, b = b, a
	}
	return [2]int{a, b}
}

// NumNodes returns the number of nodes.
func (g *Graph) NumNodes() int { g.mu.RLock(); defer g.mu.RUnlock(); return len(g.nodes) }

// NumLinks returns the number of links.
func (g *Graph) NumLinks() int { g.mu.RLock(); defer g.mu.RUnlock(); return len(g.links) }

// Links returns a copy of all links.
func (g *Graph) Links() []tomo.Link {
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make([]tomo.Link, len(g.links))
	copy(out, g.links)
	return out
}

// Nodes returns a copy of all nodes.
func (g *Graph) Nodes() []tomo.Node {
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make([]tomo.Node, len(g.nodes))
	copy(out, g.nodes)
	return out
}

// Neighbors returns node IDs adjacent to the given node.
func (g *Graph) Neighbors(nodeID int) []int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	it := g.g.From(int64(nodeID))
	var neighbors []int
	for it.Next() {
		neighbors = append(neighbors, int(it.Node().ID()))
	}
	return neighbors
}

// LinkIndex returns the link index for the edge between src and dst, or -1 if not found.
func (g *Graph) LinkIndex(src, dst int) int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.linkIndexLocked(src, dst)
}

// linkIndexLocked is the lock-free inner implementation of LinkIndex.
func (g *Graph) linkIndexLocked(src, dst int) int {
	key := linkKey(src, dst)
	if idx, ok := g.linkMap[key]; ok {
		return idx
	}
	return -1
}

// ShortestPath returns the link indices on the shortest path from src to dst.
func (g *Graph) ShortestPath(src, dst int) ([]int, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.shortestPathLocked(src, dst)
}

func (g *Graph) shortestPathLocked(src, dst int) ([]int, bool) {
	sp := path.DijkstraFrom(simple.Node(src), g.g)
	nodes, _ := sp.To(int64(dst))
	if len(nodes) < 2 {
		return nil, false
	}

	linkIDs := make([]int, 0, len(nodes)-1)
	for i := 0; i < len(nodes)-1; i++ {
		a := int(nodes[i].ID())
		b := int(nodes[i+1].ID())
		idx := g.linkIndexLocked(a, b)
		if idx < 0 {
			return nil, false
		}
		linkIDs = append(linkIDs, idx)
	}
	return linkIDs, true
}

// AllPairsShortestPaths returns path specs for all reachable (src, dst) pairs.
func (g *Graph) AllPairsShortestPaths() []tomo.PathSpec {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var paths []tomo.PathSpec
	nodeIDs := make([]int, len(g.nodes))
	for i, n := range g.nodes {
		nodeIDs[i] = n.ID
	}

	for i, src := range nodeIDs {
		for _, dst := range nodeIDs[i+1:] {
			if linkIDs, ok := g.shortestPathLocked(src, dst); ok {
				paths = append(paths, tomo.PathSpec{
					Src:     src,
					Dst:     dst,
					LinkIDs: linkIDs,
				})
			}
		}
	}
	return paths
}

// GeoDistance returns the great-circle distance in km between two nodes.
// Returns 0 if either node has no coordinates.
func GeoDistance(a, b tomo.Node) float64 {
	if (a.Latitude == 0 && a.Longitude == 0) || (b.Latitude == 0 && b.Longitude == 0) {
		return 0
	}
	const R = 6371.0 // Earth radius in km
	lat1 := a.Latitude * math.Pi / 180
	lat2 := b.Latitude * math.Pi / 180
	dlat := (b.Latitude - a.Latitude) * math.Pi / 180
	dlon := (b.Longitude - a.Longitude) * math.Pi / 180

	h := math.Sin(dlat/2)*math.Sin(dlat/2) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(dlon/2)*math.Sin(dlon/2)
	return 2 * R * math.Atan2(math.Sqrt(h), math.Sqrt(1-h))
}

// Ensure Graph implements tomo.Topology at compile time.
var _ tomo.Topology = (*Graph)(nil)

// Ensure simple.Node implements graph.Node.
var _ graph.Node = simple.Node(0)
