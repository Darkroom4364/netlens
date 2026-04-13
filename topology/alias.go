package topology

import (
	"net"

	"github.com/Darkroom4364/netlens/tomo"
)

// ResolveAliases merges nodes that likely represent different interfaces
// of the same physical router based on IP address analysis.
//
// Two IPs in the same /30 subnet but different /31 subnets, that are NOT
// directly connected, are assumed to be aliases (same router). Connected
// IPs in the same /30 are endpoints of a point-to-point link (different routers).
func ResolveAliases(g *Graph) *Graph {
	// Parse each node label as IPv4; group by /30 subnet.
	type nodeInfo struct {
		id   int
		ip   uint32
		node tomo.Node
	}
	subnet30 := make(map[uint32][]nodeInfo) // /30 key → nodes

	for _, n := range g.nodes {
		parsed := net.ParseIP(n.Label)
		if parsed == nil {
			continue
		}
		v4 := parsed.To4()
		if v4 == nil {
			continue
		}
		ip := uint32(v4[0])<<24 | uint32(v4[1])<<16 | uint32(v4[2])<<8 | uint32(v4[3])
		key := ip & 0xFFFFFFFC
		subnet30[key] = append(subnet30[key], nodeInfo{id: n.ID, ip: ip, node: n})
	}

	// Build alias map: merged node ID → representative node ID.
	rep := make(map[int]int) // nodeID → representative
	for _, group := range subnet30 {
		if len(group) < 2 {
			continue
		}
		// Within each /30 group, merge nodes in different /31s that are not connected.
		for i := 0; i < len(group); i++ {
			for j := i + 1; j < len(group); j++ {
				a, b := group[i], group[j]
				if a.ip&0xFFFFFFFE == b.ip&0xFFFFFFFE {
					continue // same /31 → different routers
				}
				if g.LinkIndex(a.id, b.id) >= 0 {
					continue // directly connected → different routers
				}
				// Merge b into a's representative.
				ra := resolve(rep, a.id)
				rep[b.id] = ra
			}
		}
	}

	// Build new graph with merged nodes.
	out := New()
	added := make(map[int]bool)
	for _, n := range g.nodes {
		r := resolve(rep, n.ID)
		if !added[r] {
			// Use the representative's original node data.
			out.AddNode(g.nodes[g.nodeMap[r]])
			added[r] = true
		}
	}
	for _, l := range g.links {
		src := resolve(rep, l.Src)
		dst := resolve(rep, l.Dst)
		if src != dst {
			out.AddLink(src, dst)
		}
	}
	return out
}

// resolve follows the representative chain to the root.
func resolve(rep map[int]int, id int) int {
	for {
		r, ok := rep[id]
		if !ok {
			return id
		}
		id = r
	}
}
