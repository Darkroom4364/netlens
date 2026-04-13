package topology

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/Darkroom4364/netlens/tomo"
)

// ASNResolver maps IP addresses to AS numbers using the RIPE RIStat API.
type ASNResolver struct {
	mu     sync.Mutex
	cache  map[string]uint32
	client *http.Client
}

// NewASNResolver creates an ASNResolver with an in-memory cache and a
// reasonably-timeboxed HTTP client.
func NewASNResolver() *ASNResolver {
	return &ASNResolver{
		cache: make(map[string]uint32),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ripeResponse models the subset of the RIPE RIStat network-info response we
// care about: {"data":{"asns":["15169"]}}.
type ripeResponse struct {
	Data struct {
		ASNs []string `json:"asns"`
	} `json:"data"`
}

// Resolve returns the ASN for an IP address.
// It checks the in-memory cache first and falls back to the RIPE RIStat API.
// Unresolvable IPs (errors, empty responses) are cached as ASN 0.
func (r *ASNResolver) Resolve(ctx context.Context, ip string) (uint32, error) {
	r.mu.Lock()
	if asn, ok := r.cache[ip]; ok {
		r.mu.Unlock()
		return asn, nil
	}
	r.mu.Unlock()

	url := fmt.Sprintf("https://stat.ripe.net/data/network-info/data.json?resource=%s", ip)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		r.cacheStore(ip, 0)
		return 0, fmt.Errorf("asn: create request for %s: %w", ip, err)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		r.cacheStore(ip, 0)
		return 0, fmt.Errorf("asn: query RIPE for %s: %w", ip, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		r.cacheStore(ip, 0)
		return 0, fmt.Errorf("asn: RIPE returned status %d for %s", resp.StatusCode, ip)
	}

	var rr ripeResponse
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		r.cacheStore(ip, 0)
		return 0, fmt.Errorf("asn: decode RIPE response for %s: %w", ip, err)
	}

	if len(rr.Data.ASNs) == 0 {
		r.cacheStore(ip, 0)
		return 0, nil
	}

	asn64, err := strconv.ParseUint(rr.Data.ASNs[0], 10, 32)
	if err != nil {
		r.cacheStore(ip, 0)
		return 0, fmt.Errorf("asn: parse ASN %q for %s: %w", rr.Data.ASNs[0], ip, err)
	}

	asn := uint32(asn64)
	r.cacheStore(ip, asn)
	return asn, nil
}

func (r *ASNResolver) cacheStore(ip string, asn uint32) {
	r.mu.Lock()
	r.cache[ip] = asn
	r.mu.Unlock()
}

// ResolveAll resolves every unique IP found in the measurements and returns an
// ip-to-ASN map. Errors on individual IPs are logged as ASN 0 and do not fail
// the whole batch.
func (r *ASNResolver) ResolveAll(ctx context.Context, measurements []tomo.PathMeasurement) (map[string]uint32, error) {
	unique := make(map[string]struct{})
	for _, m := range measurements {
		for _, h := range m.Hops {
			if !h.Anonymous && h.IP != "" {
				unique[h.IP] = struct{}{}
			}
		}
	}

	result := make(map[string]uint32, len(unique))
	for ip := range unique {
		asn, _ := r.Resolve(ctx, ip) // best-effort; failures cached as 0
		result[ip] = asn
	}
	return result, nil
}

// BuildASGraph creates a graph where nodes are ASes instead of individual IPs.
// Consecutive hops that belong to different ASes produce an AS-level link.
// Hops whose ASN is 0 (unresolved) are skipped, similar to anonymous hops in
// InferFromMeasurements.
func BuildASGraph(measurements []tomo.PathMeasurement, ipToASN map[string]uint32, opts InferOpts) (*Graph, []tomo.PathSpec, error) {
	if len(measurements) == 0 {
		return nil, nil, fmt.Errorf("topology: no measurements provided")
	}

	maxAnonFrac := opts.MaxAnonymousFrac
	if maxAnonFrac == 0 {
		maxAnonFrac = defaultMaxAnonymousFrac
	}

	g := New()

	asnToNodeID := make(map[uint32]int)
	nextNodeID := 0

	ensureNode := func(asn uint32) int {
		if id, ok := asnToNodeID[asn]; ok {
			return id
		}
		id := nextNodeID
		nextNodeID++
		asnToNodeID[asn] = id
		g.AddNode(tomo.Node{ID: id, Label: fmt.Sprintf("AS%d", asn)})
		return id
	}

	var pathSpecs []tomo.PathSpec

	for _, m := range measurements {
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
		if float64(anonCount)/float64(len(m.Hops)) > maxAnonFrac {
			continue
		}

		// Map hops to AS-level nodes, collapsing consecutive hops in the
		// same AS into a single visit.
		var asnNodeIDs []int
		var prevASN uint32
		for _, h := range m.Hops {
			if h.Anonymous || h.MPLS || h.IP == "" {
				continue
			}
			asn := ipToASN[h.IP]
			if asn == 0 {
				continue // unresolved — skip
			}
			if asn == prevASN {
				continue // still inside the same AS
			}
			prevASN = asn
			asnNodeIDs = append(asnNodeIDs, ensureNode(asn))
		}

		// Build links between consecutive AS-level nodes.
		var linkIDs []int
		for i := 0; i < len(asnNodeIDs)-1; i++ {
			src := asnNodeIDs[i]
			dst := asnNodeIDs[i+1]
			if src == dst {
				continue
			}
			linkIDs = append(linkIDs, g.AddLink(src, dst))
		}

		if len(asnNodeIDs) >= 2 {
			pathSpecs = append(pathSpecs, tomo.PathSpec{
				Src:     asnNodeIDs[0],
				Dst:     asnNodeIDs[len(asnNodeIDs)-1],
				LinkIDs: linkIDs,
			})
		}
	}

	return g, pathSpecs, nil
}
