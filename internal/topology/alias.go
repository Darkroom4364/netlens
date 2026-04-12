package topology

// ResolveAliases performs IP alias resolution on the given topology graph.
//
// Alias resolution merges multiple IP addresses that belong to the same
// physical router into a single node. The canonical approach (Kapar) groups
// IPs sharing a /30 subnet that are not directly connected — unconnected IPs
// in the same /30 are likely different interfaces on the same router, while
// connected IPs in the same /30 are endpoints of a point-to-point link
// (different routers).
//
// TODO: implement analytical Kapar alias resolution. For now this is a
// pass-through stub so the InferOpts.AliasResolution plumbing is in place.
func ResolveAliases(g *Graph) *Graph {
	return g
}
