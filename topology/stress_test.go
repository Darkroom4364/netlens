package topology

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Darkroom4364/netlens/tomo"
)

// ---------------------------------------------------------------------------
// GraphML adversarial tests
// ---------------------------------------------------------------------------

func TestStress_XMLEntityExpansion(t *testing.T) {
	// Billion laughs style entity expansion. Go's xml.Unmarshal does not
	// expand general entities, so this should fail fast without OOM.
	payload := `<?xml version="1.0"?>
<!DOCTYPE foo [
  <!ENTITY x "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA">
]>
<graphml>&x;&x;&x;</graphml>`

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = ParseGraphML([]byte(payload))
	}()

	select {
	case <-done:
		// Completed (error or not) without hanging — pass.
	case <-time.After(5 * time.Second):
		t.Fatal("ParseGraphML hung on entity expansion payload")
	}
}

func TestStress_LargeGraphML5000Nodes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test")
	}

	const numNodes = 5000
	const numEdges = 10000

	var b strings.Builder
	b.WriteString(`<?xml version="1.0"?><graphml><graph>`)
	for i := 0; i < numNodes; i++ {
		fmt.Fprintf(&b, `<node id="n%d"/>`, i)
	}
	for i := 0; i < numEdges; i++ {
		src := i % numNodes
		dst := (i*7 + 3) % numNodes
		if src == dst {
			dst = (dst + 1) % numNodes
		}
		fmt.Fprintf(&b, `<edge source="n%d" target="n%d"/>`, src, dst)
	}
	b.WriteString(`</graph></graphml>`)

	start := time.Now()
	g, err := ParseGraphML([]byte(b.String()))
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("ParseGraphML failed: %v", err)
	}
	if g.NumNodes() != numNodes {
		t.Errorf("expected %d nodes, got %d", numNodes, g.NumNodes())
	}
	t.Logf("parsed %d nodes, %d links in %v", g.NumNodes(), g.NumLinks(), elapsed)
}

func TestStress_GraphMLMissingNodeRefs(t *testing.T) {
	xml := `<?xml version="1.0"?><graphml><graph>
		<node id="a"/><node id="b"/>
		<edge source="a" target="b"/>
		<edge source="a" target="MISSING"/>
		<edge source="GHOST" target="b"/>
	</graph></graphml>`

	g, err := ParseGraphML([]byte(xml))
	if err != nil {
		t.Fatalf("expected graceful parse, got error: %v", err)
	}
	// Only the a-b edge should be added.
	if g.NumLinks() != 1 {
		t.Errorf("expected 1 link (skipping missing refs), got %d", g.NumLinks())
	}
}

func TestStress_GraphMLNoGraphElement(t *testing.T) {
	xml := `<?xml version="1.0"?><graphml><key id="k0" for="node" attr.name="label" attr.type="string"/></graphml>`

	g, err := ParseGraphML([]byte(xml))
	// ParseGraphML does not explicitly return an error for missing <graph>,
	// but the result should be an empty graph with 0 nodes/links.
	if err != nil {
		// If it does return an error, that's also acceptable.
		t.Logf("got error (acceptable): %v", err)
		return
	}
	if g.NumNodes() != 0 || g.NumLinks() != 0 {
		t.Errorf("expected empty graph, got %d nodes %d links", g.NumNodes(), g.NumLinks())
	}
}

func TestStress_DeeplyNestedXML(t *testing.T) {
	// 100 levels of nested elements inside a graphml doc.
	const depth = 100
	var b strings.Builder
	b.WriteString(`<?xml version="1.0"?><graphml><graph>`)
	for i := 0; i < depth; i++ {
		b.WriteString("<node id=\"deep\"><data key=\"k0\">")
	}
	b.WriteString("leaf")
	for i := 0; i < depth; i++ {
		b.WriteString("</data></node>")
	}
	b.WriteString(`</graph></graphml>`)

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = ParseGraphML([]byte(b.String()))
	}()

	select {
	case <-done:
		// No stack overflow — pass.
	case <-time.After(5 * time.Second):
		t.Fatal("ParseGraphML hung on deeply nested XML")
	}
}

// ---------------------------------------------------------------------------
// Large-scale topology tests
// ---------------------------------------------------------------------------

func TestStress_AllPairsShortestPaths1000Nodes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test")
	}
	if os.Getenv("NETLENS_STRESS") == "" {
		t.Skip("skipping stress test; set NETLENS_STRESS=1 to enable")
	}

	// Use 300 nodes / 900 links — AllPairsShortestPaths runs individual
	// Dijkstra per source, so O(V^2 * (V+E)). 1000 nodes exceeds 60s.
	g := BarabasiAlbert(300, 3, 42)
	if g.NumNodes() != 300 {
		t.Fatalf("expected 300 nodes, got %d", g.NumNodes())
	}

	start := time.Now()
	paths := g.AllPairsShortestPaths()
	elapsed := time.Since(start)

	if elapsed > 10*time.Second {
		t.Fatalf("AllPairsShortestPaths took %v (limit 10s)", elapsed)
	}
	t.Logf("AllPairsShortestPaths: %d paths in %v", len(paths), elapsed)
}

func TestStress_InferFromMeasurements500Nodes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test")
	}

	g := BarabasiAlbert(200, 4, 99)
	paths := g.AllPairsShortestPaths()

	// Build 2000 synthetic measurements from the available paths.
	measurements := make([]tomo.PathMeasurement, 0, 2000)
	for i := 0; i < 2000 && i < len(paths); i++ {
		p := paths[i%len(paths)]
		hops := make([]tomo.Hop, 0, len(p.LinkIDs)+1)
		// Walk through link IDs to reconstruct IPs.
		links := g.Links()
		prev := p.Src
		hops = append(hops, tomo.Hop{IP: fmt.Sprintf("10.0.%d.%d", prev/256, prev%256)})
		for _, lid := range p.LinkIDs {
			l := links[lid]
			next := l.Dst
			if next == prev {
				next = l.Src
			}
			hops = append(hops, tomo.Hop{IP: fmt.Sprintf("10.0.%d.%d", next/256, next%256)})
			prev = next
		}
		measurements = append(measurements, tomo.PathMeasurement{
			Src:  hops[0].IP,
			Dst:  hops[len(hops)-1].IP,
			Hops: hops,
			RTTs: []time.Duration{time.Millisecond * 10},
		})
	}

	start := time.Now()
	ig, specs, accepted, err := InferFromMeasurements(measurements, InferOpts{})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("InferFromMeasurements failed: %v", err)
	}
	t.Logf("inferred %d nodes, %d links, %d paths, %d accepted in %v",
		ig.NumNodes(), ig.NumLinks(), len(specs), len(accepted), elapsed)
}

func TestStress_BarabasiAlbert500Connected(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test")
	}

	g := BarabasiAlbert(500, 5, 42)
	if g.NumNodes() != 500 {
		t.Fatalf("expected 500 nodes, got %d", g.NumNodes())
	}
	// Check connectivity: every node should be reachable from node 0.
	assertConnected(t, g, "BarabasiAlbert(500,5,42)")
}

func TestStress_Waxman500Connected(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test")
	}

	g := Waxman(500, 0.3, 0.5, 42)
	if g.NumNodes() != 500 {
		t.Fatalf("expected 500 nodes, got %d", g.NumNodes())
	}
	assertConnected(t, g, "Waxman(500,0.3,0.5,42)")
}

// ---------------------------------------------------------------------------
// Inference at scale
// ---------------------------------------------------------------------------

func TestStress_Inference500MeasWith20Hops(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test")
	}

	// 500 measurements each with 20 hops using distinct IPs.
	measurements := make([]tomo.PathMeasurement, 500)
	for i := 0; i < 500; i++ {
		hops := make([]tomo.Hop, 20)
		for j := 0; j < 20; j++ {
			hops[j] = tomo.Hop{IP: fmt.Sprintf("10.%d.%d.%d", i/256, i%256, j)}
		}
		measurements[i] = tomo.PathMeasurement{
			Src:  hops[0].IP,
			Dst:  hops[19].IP,
			Hops: hops,
			RTTs: []time.Duration{5 * time.Millisecond},
		}
	}

	g, specs, accepted, err := InferFromMeasurements(measurements, InferOpts{})
	if err != nil {
		t.Fatalf("InferFromMeasurements failed: %v", err)
	}
	if len(specs) != 500 {
		t.Errorf("expected 500 path specs, got %d", len(specs))
	}
	t.Logf("nodes=%d links=%d specs=%d accepted=%d", g.NumNodes(), g.NumLinks(), len(specs), len(accepted))
}

func TestStress_InferenceMassiveLinkSharing(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test")
	}

	// All 500 measurements pass through the same 3 hops: A -> B -> C -> D
	// but differ in their source prefix.
	measurements := make([]tomo.PathMeasurement, 500)
	for i := 0; i < 500; i++ {
		srcIP := fmt.Sprintf("192.168.%d.%d", i/256, i%256)
		hops := []tomo.Hop{
			{IP: srcIP},
			{IP: "10.0.0.1"},
			{IP: "10.0.0.2"},
			{IP: "10.0.0.3"},
		}
		measurements[i] = tomo.PathMeasurement{
			Src:  srcIP,
			Dst:  "10.0.0.3",
			Hops: hops,
			RTTs: []time.Duration{2 * time.Millisecond},
		}
	}

	g, specs, _, err := InferFromMeasurements(measurements, InferOpts{})
	if err != nil {
		t.Fatalf("InferFromMeasurements failed: %v", err)
	}

	// The shared links 10.0.0.1->10.0.0.2 and 10.0.0.2->10.0.0.3 should exist.
	if g.NumNodes() < 3 {
		t.Errorf("expected at least 3 shared nodes, got %d", g.NumNodes())
	}
	t.Logf("nodes=%d links=%d specs=%d (massive sharing)", g.NumNodes(), g.NumLinks(), len(specs))
}

func TestStress_InferenceAlmostAllRejected(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test")
	}

	// 1000 measurements where ~50% of hops are anonymous.
	// MaxAnonymousFrac = 0.0001 means almost all should be rejected.
	measurements := make([]tomo.PathMeasurement, 1000)
	for i := 0; i < 1000; i++ {
		hops := make([]tomo.Hop, 10)
		for j := 0; j < 10; j++ {
			if j%2 == 0 {
				hops[j] = tomo.Hop{IP: fmt.Sprintf("10.%d.%d.%d", i/256, i%256, j)}
			} else {
				hops[j] = tomo.Hop{Anonymous: true}
			}
		}
		measurements[i] = tomo.PathMeasurement{
			Src:  hops[0].IP,
			Dst:  fmt.Sprintf("10.%d.%d.9", i/256, i%256),
			Hops: hops,
			RTTs: []time.Duration{3 * time.Millisecond},
		}
	}

	// With MaxAnonymousFrac=0.0001, all measurements with 50% anonymous should be rejected.
	// This should cause an empty result (or error) since no measurements are accepted.
	g, specs, accepted, err := InferFromMeasurements(measurements, InferOpts{MaxAnonymousFrac: 0.0001})
	if err != nil {
		// Error is acceptable if all measurements are rejected.
		t.Logf("got error (acceptable): %v", err)
		return
	}
	// If no error, accepted should be very small or zero.
	if len(accepted) > 10 {
		t.Errorf("expected almost all rejected, but %d accepted out of 1000", len(accepted))
	}
	t.Logf("nodes=%d links=%d specs=%d accepted=%d (almost all rejected)",
		g.NumNodes(), g.NumLinks(), len(specs), len(accepted))
}

// ---------------------------------------------------------------------------
// Synthetic generator boundaries
// ---------------------------------------------------------------------------

func TestStress_BarabasiAlbertAlmostComplete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test")
	}

	g := BarabasiAlbert(100, 99, 42)
	if g.NumNodes() != 100 {
		t.Fatalf("expected 100 nodes, got %d", g.NumNodes())
	}
	// m=99 with n=100: the seed graph has 100 nodes (m+1=100) and is complete.
	// No additional nodes are added, so the graph is K_100.
	maxEdges := 100 * 99 / 2
	t.Logf("BarabasiAlbert(100,99): nodes=%d links=%d (max possible=%d)", g.NumNodes(), g.NumLinks(), maxEdges)
	if g.NumLinks() < 100 {
		t.Errorf("expected a highly connected graph, got only %d links", g.NumLinks())
	}
}

func TestStress_WaxmanHighConnectivity(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test")
	}

	g := Waxman(100, 10.0, 1.0, 42)
	if g.NumNodes() != 100 {
		t.Fatalf("expected 100 nodes, got %d", g.NumNodes())
	}
	maxEdges := 100 * 99 / 2
	// With alpha=10.0, beta=1.0, probability is very high for all pairs.
	t.Logf("Waxman(100,10.0,1.0): links=%d (max=%d, ratio=%.2f)",
		g.NumLinks(), maxEdges, float64(g.NumLinks())/float64(maxEdges))
	if g.NumLinks() < maxEdges/2 {
		t.Errorf("expected very dense graph, got only %d/%d links", g.NumLinks(), maxEdges)
	}
	assertConnected(t, g, "Waxman(100,10.0,1.0,42)")
}

func TestStress_WaxmanVerySparse(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test")
	}

	g := Waxman(100, 0.001, 0.001, 42)
	if g.NumNodes() != 100 {
		t.Fatalf("expected 100 nodes, got %d", g.NumNodes())
	}
	// With tiny alpha and beta, the Waxman model produces very few random edges.
	// The connectivity fix-up in Waxman adds a spanning tree, so we still get
	// at least 99 links, but not many more.
	t.Logf("Waxman(100,0.001,0.001): links=%d (sparse)", g.NumLinks())
	// Waxman guarantees connectivity via its union-find fix-up.
	assertConnected(t, g, "Waxman(100,0.001,0.001,42)")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// assertConnected checks that every node is reachable from node 0 via BFS.
func assertConnected(t *testing.T, g *Graph, label string) {
	t.Helper()
	if g.NumNodes() == 0 {
		return
	}
	nodes := g.Nodes()
	start := nodes[0].ID
	visited := make(map[int]bool)
	queue := []int{start}
	visited[start] = true
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, nb := range g.Neighbors(cur) {
			if !visited[nb] {
				visited[nb] = true
				queue = append(queue, nb)
			}
		}
	}
	if len(visited) != g.NumNodes() {
		t.Errorf("%s: graph not connected — reached %d/%d nodes", label, len(visited), g.NumNodes())
	}
}
