package topology

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/Darkroom4364/netlens/tomo"
)

// redirectTransport intercepts all outgoing requests and redirects them to the
// httptest server, preserving the query string.  This lets us test the
// ASNResolver without touching its hardcoded RIPE URL.
type redirectTransport struct {
	target *url.URL        // mock server base URL
	inner  http.RoundTripper
}

func (t *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme = t.target.Scheme
	req.URL.Host = t.target.Host
	// keep path + query
	return t.inner.RoundTrip(req)
}

// newTestResolver creates an ASNResolver whose HTTP client redirects all
// requests to the given httptest server.
func newTestResolver(srv *httptest.Server) *ASNResolver {
	u, _ := url.Parse(srv.URL)
	r := NewASNResolver()
	r.client = &http.Client{
		Transport: &redirectTransport{
			target: u,
			inner:  srv.Client().Transport,
		},
	}
	return r
}

// ---------------------------------------------------------------------------
// ASNResolver.Resolve tests
// ---------------------------------------------------------------------------

func TestASNResolve_Valid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"data":{"asns":["15169"]}}`)
	}))
	defer srv.Close()

	res := newTestResolver(srv)
	asn, err := res.Resolve(context.Background(), "8.8.8.8")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if asn != 15169 {
		t.Errorf("got ASN %d, want 15169", asn)
	}
}

func TestASNResolve_CacheHit(t *testing.T) {
	var reqCount int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&reqCount, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"data":{"asns":["15169"]}}`)
	}))
	defer srv.Close()

	res := newTestResolver(srv)
	ctx := context.Background()

	// First call — should hit the server.
	if _, err := res.Resolve(ctx, "8.8.8.8"); err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	// Second call — should be served from cache.
	asn, err := res.Resolve(ctx, "8.8.8.8")
	if err != nil {
		t.Fatalf("second resolve: %v", err)
	}
	if asn != 15169 {
		t.Errorf("got ASN %d, want 15169", asn)
	}
	if got := atomic.LoadInt64(&reqCount); got != 1 {
		t.Errorf("HTTP requests = %d, want 1 (cache miss only)", got)
	}
}

func TestASNResolve_InvalidIP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"data":{"asns":[]}}`)
	}))
	defer srv.Close()

	res := newTestResolver(srv)
	asn, err := res.Resolve(context.Background(), "999.999.999.999")
	if err != nil {
		t.Fatalf("expected no error for invalid IP, got: %v", err)
	}
	if asn != 0 {
		t.Errorf("got ASN %d, want 0", asn)
	}
}

func TestASNResolve_HTTP500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `internal server error`)
	}))
	defer srv.Close()

	res := newTestResolver(srv)
	asn, _ := res.Resolve(context.Background(), "1.2.3.4")
	if asn != 0 {
		t.Errorf("got ASN %d, want 0 on HTTP 500", asn)
	}
}

func TestASNResolve_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{not json at all!!!`)
	}))
	defer srv.Close()

	res := newTestResolver(srv)
	asn, _ := res.Resolve(context.Background(), "1.2.3.4")
	if asn != 0 {
		t.Errorf("got ASN %d, want 0 on malformed JSON", asn)
	}
}

func TestASNResolve_EmptyASNs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"data":{"asns":[]}}`)
	}))
	defer srv.Close()

	res := newTestResolver(srv)
	asn, err := res.Resolve(context.Background(), "10.0.0.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if asn != 0 {
		t.Errorf("got ASN %d, want 0 for empty asns", asn)
	}
}

func TestASNResolve_MultipleASNs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"data":{"asns":["15169","13335"]}}`)
	}))
	defer srv.Close()

	res := newTestResolver(srv)
	asn, err := res.Resolve(context.Background(), "8.8.8.8")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if asn != 15169 {
		t.Errorf("got ASN %d, want 15169 (first in list)", asn)
	}
}

// ---------------------------------------------------------------------------
// ASNResolver.ResolveAll tests
// ---------------------------------------------------------------------------

func TestASNResolveAll_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("no HTTP request expected for empty measurements")
	}))
	defer srv.Close()

	res := newTestResolver(srv)
	m, err := res.ResolveAll(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m) != 0 {
		t.Errorf("expected empty map, got %d entries", len(m))
	}
}

func TestASNResolveAll_DuplicateIPs(t *testing.T) {
	var reqCount int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&reqCount, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"data":{"asns":["15169"]}}`)
	}))
	defer srv.Close()

	res := newTestResolver(srv)
	measurements := []tomo.PathMeasurement{
		{Hops: []tomo.Hop{{IP: "8.8.8.8"}, {IP: "8.8.4.4"}}},
		{Hops: []tomo.Hop{{IP: "8.8.8.8"}, {IP: "8.8.4.4"}}},
	}

	m, err := res.ResolveAll(context.Background(), measurements)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m) != 2 {
		t.Errorf("expected 2 unique IPs, got %d", len(m))
	}
	// Two unique IPs, each resolved once — but the resolver's own cache
	// means the HTTP server should see exactly 2 requests.
	if got := atomic.LoadInt64(&reqCount); got != 2 {
		t.Errorf("HTTP requests = %d, want 2 (one per unique IP)", got)
	}
}

func TestASNResolveAll_AllAnonymous(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("no HTTP request expected when all hops are anonymous")
	}))
	defer srv.Close()

	res := newTestResolver(srv)
	measurements := []tomo.PathMeasurement{
		{Hops: []tomo.Hop{{Anonymous: true}, {Anonymous: true}}},
		{Hops: []tomo.Hop{{Anonymous: true}}},
	}

	m, err := res.ResolveAll(context.Background(), measurements)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m) != 0 {
		t.Errorf("expected empty map, got %d entries", len(m))
	}
}

// ---------------------------------------------------------------------------
// BuildASGraph tests
// ---------------------------------------------------------------------------

func TestASNBuildASGraph_SameASN(t *testing.T) {
	// Two hops in the same AS should collapse to a single node.
	measurements := []tomo.PathMeasurement{
		{
			Src: "1.1.1.1",
			Dst: "1.1.1.2",
			Hops: []tomo.Hop{
				{IP: "1.1.1.1"},
				{IP: "1.1.1.2"},
			},
		},
	}
	ipToASN := map[string]uint32{
		"1.1.1.1": 13335,
		"1.1.1.2": 13335,
	}

	g, paths, err := BuildASGraph(measurements, ipToASN, InferOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.NumNodes() != 1 {
		t.Errorf("nodes = %d, want 1 (same AS collapsed)", g.NumNodes())
	}
	// With a single node, no link and no path spec.
	if g.NumLinks() != 0 {
		t.Errorf("links = %d, want 0", g.NumLinks())
	}
	if len(paths) != 0 {
		t.Errorf("paths = %d, want 0 (single-node path not emitted)", len(paths))
	}
}

func TestASNBuildASGraph_DifferentASNs(t *testing.T) {
	measurements := []tomo.PathMeasurement{
		{
			Src: "8.8.8.8",
			Dst: "1.1.1.1",
			Hops: []tomo.Hop{
				{IP: "8.8.8.8"},
				{IP: "1.1.1.1"},
			},
		},
	}
	ipToASN := map[string]uint32{
		"8.8.8.8": 15169,
		"1.1.1.1": 13335,
	}

	g, paths, err := BuildASGraph(measurements, ipToASN, InferOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.NumNodes() != 2 {
		t.Errorf("nodes = %d, want 2", g.NumNodes())
	}
	if g.NumLinks() != 1 {
		t.Errorf("links = %d, want 1", g.NumLinks())
	}
	if len(paths) != 1 {
		t.Errorf("paths = %d, want 1", len(paths))
	}
}

func TestASNBuildASGraph_EmptyMeasurements(t *testing.T) {
	_, _, err := BuildASGraph(nil, nil, InferOpts{})
	if err == nil {
		t.Fatal("expected error for empty measurements")
	}
}

func TestASNBuildASGraph_AllUnresolved(t *testing.T) {
	measurements := []tomo.PathMeasurement{
		{
			Hops: []tomo.Hop{
				{IP: "10.0.0.1"},
				{IP: "10.0.0.2"},
			},
		},
	}
	// All IPs map to ASN 0 (unresolved).
	ipToASN := map[string]uint32{
		"10.0.0.1": 0,
		"10.0.0.2": 0,
	}

	g, paths, err := BuildASGraph(measurements, ipToASN, InferOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.NumNodes() != 0 {
		t.Errorf("nodes = %d, want 0 (all unresolved)", g.NumNodes())
	}
	if len(paths) != 0 {
		t.Errorf("paths = %d, want 0", len(paths))
	}
}

func TestASNBuildASGraph_MaxAnonymousFrac(t *testing.T) {
	// Path with 2/3 anonymous hops (66%) should be dropped at threshold 0.5.
	measurements := []tomo.PathMeasurement{
		{
			Hops: []tomo.Hop{
				{IP: "8.8.8.8"},
				{Anonymous: true},
				{Anonymous: true},
			},
		},
	}
	ipToASN := map[string]uint32{
		"8.8.8.8": 15169,
	}

	g, paths, err := BuildASGraph(measurements, ipToASN, InferOpts{MaxAnonymousFrac: 0.5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The measurement exceeds the threshold and is skipped entirely.
	if g.NumNodes() != 0 {
		t.Errorf("nodes = %d, want 0 (path exceeded anon threshold)", g.NumNodes())
	}
	if len(paths) != 0 {
		t.Errorf("paths = %d, want 0", len(paths))
	}
}
