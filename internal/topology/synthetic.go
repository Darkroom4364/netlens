package topology

import (
	"fmt"
	"math"
	"math/rand"

	"github.com/Darkroom4364/netlens/internal/tomo"
)

// BarabasiAlbert generates a scale-free graph using preferential attachment.
// Starts with a complete graph of m+1 nodes, then attaches each new node to m existing nodes.
func BarabasiAlbert(n, m int, seed int64) *Graph {
	rng := rand.New(rand.NewSource(seed))
	g := New()

	// Seed complete graph of m+1 nodes.
	for i := 0; i <= m; i++ {
		g.AddNode(tomo.Node{ID: i, Label: fmt.Sprintf("n%d", i), Latitude: rng.Float64() * 100, Longitude: rng.Float64() * 100})
	}
	for i := 0; i <= m; i++ {
		for j := i + 1; j <= m; j++ {
			g.AddLink(i, j)
		}
	}

	// Grow by preferential attachment.
	for id := m + 1; id < n; id++ {
		g.AddNode(tomo.Node{ID: id, Label: fmt.Sprintf("n%d", id), Latitude: rng.Float64() * 100, Longitude: rng.Float64() * 100})
		targets := make(map[int]bool)
		totalDeg := 2 * g.NumLinks() // sum of degrees in undirected graph
		for len(targets) < m {
			cumul := 0
			r := rng.Intn(totalDeg)
			for _, l := range g.Links() {
				cumul++
				if cumul > r && !targets[l.Src] && l.Src != id {
					targets[l.Src] = true
					break
				}
				cumul++
				if cumul > r && !targets[l.Dst] && l.Dst != id {
					targets[l.Dst] = true
					break
				}
			}
		}
		for t := range targets {
			g.AddLink(id, t)
		}
	}
	return g
}

// Waxman generates a random geographic graph. Nodes are placed uniformly in [0,1]x[0,1].
// Each pair is connected with probability beta*exp(-d/(alpha*L)) where L is max distance.
func Waxman(n int, alpha, beta float64, seed int64) *Graph {
	rng := rand.New(rand.NewSource(seed))
	g := New()

	xs := make([]float64, n)
	ys := make([]float64, n)
	for i := 0; i < n; i++ {
		xs[i], ys[i] = rng.Float64(), rng.Float64()
		g.AddNode(tomo.Node{ID: i, Label: fmt.Sprintf("n%d", i), Latitude: xs[i], Longitude: ys[i]})
	}

	dist := func(i, j int) float64 {
		dx, dy := xs[i]-xs[j], ys[i]-ys[j]
		return math.Sqrt(dx*dx + dy*dy)
	}

	// Find max pairwise distance L.
	L := 0.0
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if d := dist(i, j); d > L {
				L = d
			}
		}
	}

	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			p := beta * math.Exp(-dist(i, j)/(alpha*L))
			if rng.Float64() < p {
				g.AddLink(i, j)
			}
		}
	}

	// Ensure connectivity via union-find.
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	for _, l := range g.Links() {
		a, b := find(l.Src), find(l.Dst)
		if a != b {
			parent[a] = b
		}
	}
	root := find(0)
	for i := 1; i < n; i++ {
		if find(i) != root {
			g.AddLink(i-1, i)
			parent[find(i)] = root
		}
	}
	return g
}
