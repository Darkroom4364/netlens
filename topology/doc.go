// Package topology provides network graph representations and construction
// utilities for network tomography.
//
// Graph is the primary type: an undirected network backed by gonum, with
// support for loading Topology Zoo GraphML files, generating synthetic
// topologies (Barabási-Albert, Waxman), and inferring topology from
// traceroute measurements.
//
//	import "github.com/Darkroom4364/netlens/topology"
//
//	g, _ := topology.LoadGraphML("abilene.graphml")
//	paths := g.AllPairsShortestPaths()
package topology
