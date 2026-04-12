package measure

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// PerfSONAR mock data
// ---------------------------------------------------------------------------

var mockPerfSONARMetadata = `[
  {
    "source": "10.0.0.1",
    "destination": "10.0.0.2",
    "event-types": [
      {
        "event-type": "packet-trace",
        "base-uri": "/esmond/perfsonar/archive/abcd1234/packet-trace/base"
      },
      {
        "event-type": "histogram-owdelay",
        "base-uri": "/esmond/perfsonar/archive/abcd1234/histogram-owdelay/base"
      }
    ]
  }
]`

var mockPerfSONARTimeseries = `[
  {"ts": 1700000000, "val": 5.3},
  {"ts": 1700000060, "val": 6.1},
  {"ts": 1700000120, "val": 4.9}
]`

var mockPerfSONARMetadataNoTrace = `[
  {
    "source": "10.0.0.1",
    "destination": "10.0.0.2",
    "event-types": [
      {
        "event-type": "histogram-owdelay",
        "base-uri": "/esmond/perfsonar/archive/abcd1234/histogram-owdelay/base"
      }
    ]
  }
]`

// newPerfSONARMockServer creates an httptest.Server mimicking the esmond API.
func newPerfSONARMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		// Timeseries endpoint.
		case strings.HasPrefix(r.URL.Path, "/esmond/perfsonar/archive/abcd1234/packet-trace/base"):
			fmt.Fprint(w, mockPerfSONARTimeseries)

		// Metadata endpoint (archive root).
		case r.URL.Path == "/esmond/perfsonar/archive":
			q := r.URL.Query()
			if q.Get("source") == "no-trace" {
				fmt.Fprint(w, mockPerfSONARMetadataNoTrace)
				return
			}
			if q.Get("source") == "empty-ts" {
				// Return metadata that points to the empty-ts timeseries.
				fmt.Fprint(w, `[{"source":"10.0.0.1","destination":"10.0.0.2","event-types":[{"event-type":"packet-trace","base-uri":"/esmond/perfsonar/archive/empty-ts/packet-trace/base"}]}]`)
				return
			}
			if q.Get("source") == "negative" {
				fmt.Fprint(w, `[{"source":"10.0.0.1","destination":"10.0.0.2","event-types":[{"event-type":"packet-trace","base-uri":"/esmond/perfsonar/archive/negative/packet-trace/base"}]}]`)
				return
			}
			fmt.Fprint(w, mockPerfSONARMetadata)

		// Empty timeseries.
		case strings.HasPrefix(r.URL.Path, "/esmond/perfsonar/archive/empty-ts/"):
			fmt.Fprint(w, `[]`)

		// Negative latency values.
		case strings.HasPrefix(r.URL.Path, "/esmond/perfsonar/archive/negative/"):
			fmt.Fprint(w, `[{"ts":1700000000,"val":-3.5},{"ts":1700000060,"val":-0.1}]`)

		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `{"error":"not found: %s"}`, r.URL.Path)
		}
	}))
}

// ---------------------------------------------------------------------------
// PerfSONAR tests
// ---------------------------------------------------------------------------

func TestAdapterPerfSONAR_SuccessfulFetch(t *testing.T) {
	srv := newPerfSONARMockServer(t)
	defer srv.Close()

	src := NewPerfSONARSource(srv.URL+"/esmond/perfsonar/archive", srv.Client())
	results, err := src.FetchLatency(context.Background(), "10.0.0.1", "10.0.0.2", 3600)
	if err != nil {
		t.Fatalf("FetchLatency: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}

	if results[0].Src != "10.0.0.1" {
		t.Errorf("Src = %q, want 10.0.0.1", results[0].Src)
	}
	if results[0].Dst != "10.0.0.2" {
		t.Errorf("Dst = %q, want 10.0.0.2", results[0].Dst)
	}
	if results[0].Timestamp.Unix() != 1700000000 {
		t.Errorf("Timestamp = %v, want unix 1700000000", results[0].Timestamp)
	}
	if results[0].Weight != 1.0 {
		t.Errorf("Weight = %f, want 1.0", results[0].Weight)
	}

	// 5.3ms expected for first point.
	wantRTT := time.Duration(5.3 * float64(time.Millisecond))
	if len(results[0].RTTs) != 1 || results[0].RTTs[0] != wantRTT {
		t.Errorf("RTTs[0] = %v, want %v", results[0].RTTs, wantRTT)
	}
}

func TestAdapterPerfSONAR_NoMatchingEventType(t *testing.T) {
	srv := newPerfSONARMockServer(t)
	defer srv.Close()

	src := NewPerfSONARSource(srv.URL+"/esmond/perfsonar/archive", srv.Client())
	_, err := src.FetchLatency(context.Background(), "no-trace", "10.0.0.2", 3600)
	if err == nil {
		t.Fatal("expected error when no packet-trace event-type exists")
	}
	if !strings.Contains(err.Error(), "no packet-trace") {
		t.Errorf("error = %q, want it to mention 'no packet-trace'", err)
	}
}

func TestAdapterPerfSONAR_EmptyTimeseries(t *testing.T) {
	srv := newPerfSONARMockServer(t)
	defer srv.Close()

	src := NewPerfSONARSource(srv.URL+"/esmond/perfsonar/archive", srv.Client())
	results, err := src.FetchLatency(context.Background(), "empty-ts", "10.0.0.2", 3600)
	if err != nil {
		t.Fatalf("FetchLatency: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0", len(results))
	}
}

func TestAdapterPerfSONAR_HTTP404OnMetadata(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":"not found"}`)
	}))
	defer srv.Close()

	src := NewPerfSONARSource(srv.URL+"/esmond/perfsonar/archive", srv.Client())
	_, err := src.FetchLatency(context.Background(), "10.0.0.1", "10.0.0.2", 3600)
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error = %q, want it to mention 404", err)
	}
}

func TestAdapterPerfSONAR_HTTP500OnTimeseries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "packet-trace/base") {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"error":"internal server error"}`)
			return
		}
		// Serve metadata normally.
		fmt.Fprint(w, mockPerfSONARMetadata)
	}))
	defer srv.Close()

	src := NewPerfSONARSource(srv.URL+"/esmond/perfsonar/archive", srv.Client())
	_, err := src.FetchLatency(context.Background(), "10.0.0.1", "10.0.0.2", 3600)
	if err == nil {
		t.Fatal("expected error for 500 on timeseries fetch")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error = %q, want it to mention 500", err)
	}
}

func TestAdapterPerfSONAR_RetryOn429(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Only rate-limit the metadata call, let timeseries pass through.
		if !strings.Contains(r.URL.Path, "packet-trace/base") {
			count := atomic.AddInt32(&attempts, 1)
			if count <= 2 {
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(http.StatusTooManyRequests)
				fmt.Fprint(w, `{"error":"rate limited"}`)
				return
			}
			fmt.Fprint(w, mockPerfSONARMetadata)
			return
		}
		fmt.Fprint(w, mockPerfSONARTimeseries)
	}))
	defer srv.Close()

	src := NewPerfSONARSource(srv.URL+"/esmond/perfsonar/archive", srv.Client())
	results, err := src.FetchLatency(context.Background(), "10.0.0.1", "10.0.0.2", 3600)
	if err != nil {
		t.Fatalf("FetchLatency after retries: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("got %d results, want 3", len(results))
	}

	got := atomic.LoadInt32(&attempts)
	if got < 3 {
		t.Errorf("metadata attempts = %d, want >= 3 (2 retries + 1 success)", got)
	}
}

func TestAdapterPerfSONAR_MalformedJSONMetadata(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{not valid json`)
	}))
	defer srv.Close()

	src := NewPerfSONARSource(srv.URL+"/esmond/perfsonar/archive", srv.Client())
	_, err := src.FetchLatency(context.Background(), "10.0.0.1", "10.0.0.2", 3600)
	if err == nil {
		t.Fatal("expected error for malformed JSON metadata")
	}
	if !strings.Contains(err.Error(), "parse metadata") {
		t.Errorf("error = %q, want it to mention 'parse metadata'", err)
	}
}

func TestAdapterPerfSONAR_MalformedJSONTimeseries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "packet-trace/base") {
			fmt.Fprint(w, `[{"ts": broken}]`)
			return
		}
		fmt.Fprint(w, mockPerfSONARMetadata)
	}))
	defer srv.Close()

	src := NewPerfSONARSource(srv.URL+"/esmond/perfsonar/archive", srv.Client())
	_, err := src.FetchLatency(context.Background(), "10.0.0.1", "10.0.0.2", 3600)
	if err == nil {
		t.Fatal("expected error for malformed JSON timeseries")
	}
	if !strings.Contains(err.Error(), "parse timeseries") {
		t.Errorf("error = %q, want it to mention 'parse timeseries'", err)
	}
}

func TestAdapterPerfSONAR_NegativeLatency(t *testing.T) {
	srv := newPerfSONARMockServer(t)
	defer srv.Close()

	src := NewPerfSONARSource(srv.URL+"/esmond/perfsonar/archive", srv.Client())
	results, err := src.FetchLatency(context.Background(), "negative", "10.0.0.2", 3600)
	if err != nil {
		t.Fatalf("FetchLatency: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}

	// Negative values should still parse; -3.5ms.
	if results[0].RTTs[0] >= 0 {
		t.Errorf("RTTs[0] = %v, expected negative duration for val=-3.5", results[0].RTTs[0])
	}
}

func TestAdapterPerfSONAR_TrailingSlashBaseURL(t *testing.T) {
	srv := newPerfSONARMockServer(t)
	defer srv.Close()

	// With trailing slash.
	srcSlash := NewPerfSONARSource(srv.URL+"/esmond/perfsonar/archive/", srv.Client())
	results1, err := srcSlash.FetchLatency(context.Background(), "10.0.0.1", "10.0.0.2", 3600)
	if err != nil {
		t.Fatalf("FetchLatency (trailing slash): %v", err)
	}

	// Without trailing slash.
	srcNoSlash := NewPerfSONARSource(srv.URL+"/esmond/perfsonar/archive", srv.Client())
	results2, err := srcNoSlash.FetchLatency(context.Background(), "10.0.0.1", "10.0.0.2", 3600)
	if err != nil {
		t.Fatalf("FetchLatency (no trailing slash): %v", err)
	}

	if len(results1) != len(results2) {
		t.Errorf("trailing slash gave %d results, no slash gave %d — should be identical",
			len(results1), len(results2))
	}
}

// ---------------------------------------------------------------------------
// ICMP prober tests (struct construction + edge cases, no network)
// ---------------------------------------------------------------------------

func TestAdapterICMP_DefaultValues(t *testing.T) {
	p := NewICMPProber()

	if p.MaxHops != 32 {
		t.Errorf("MaxHops = %d, want 32", p.MaxHops)
	}
	if p.Timeout != 2*time.Second {
		t.Errorf("Timeout = %v, want 2s", p.Timeout)
	}
	if p.Count != 3 {
		t.Errorf("Count = %d, want 3", p.Count)
	}
}

func TestAdapterICMP_ProbeMultipleEmptyTargets(t *testing.T) {
	p := NewICMPProber()
	results, err := p.ProbeMultiple(context.Background(), []string{})
	if err != nil {
		t.Fatalf("ProbeMultiple with empty targets: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0", len(results))
	}
}

func TestAdapterICMP_ProbeMultipleNilTargets(t *testing.T) {
	p := NewICMPProber()
	results, err := p.ProbeMultiple(context.Background(), nil)
	if err != nil {
		t.Fatalf("ProbeMultiple with nil targets: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0", len(results))
	}
}

func TestAdapterICMP_ProbeMultipleNilContext(t *testing.T) {
	p := NewICMPProber()
	// nil context causes errgroup.WithContext to panic. Verify that
	// the panic is deterministic (not a nil-pointer in our code).
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic from nil context, got none")
		}
	}()
	// This should panic inside errgroup.WithContext.
	_, _ = p.ProbeMultiple(nil, []string{"127.0.0.1"})
}

func TestAdapterICMP_MaxHopsZero(t *testing.T) {
	p := &ICMPProber{
		MaxHops: 0,
		Timeout: 2 * time.Second,
		Count:   3,
	}
	// MaxHops=0 means the for loop (ttl=1; ttl<=0) never executes.
	// Probe should return immediately without infinite looping.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Use a non-routable address; we expect the loop body to never run
	// so pinger is never created. This tests the loop guard only.
	m, err := p.Probe(ctx, "127.0.0.1")
	if err != nil {
		t.Fatalf("Probe with MaxHops=0: %v", err)
	}
	if len(m.Hops) != 0 {
		t.Errorf("got %d hops, want 0 for MaxHops=0", len(m.Hops))
	}
}

func TestAdapterICMP_CountZero(t *testing.T) {
	p := &ICMPProber{
		MaxHops: 32,
		Timeout: 2 * time.Second,
		Count:   0,
	}
	// Count=0 means pinger.Count=0. The struct should construct fine.
	if p.Count != 0 {
		t.Errorf("Count = %d, want 0", p.Count)
	}
}

func TestAdapterICMP_NegativeTimeout(t *testing.T) {
	p := &ICMPProber{
		MaxHops: 32,
		Timeout: -1 * time.Second,
		Count:   3,
	}
	// Negative timeout should be stored without panic on construction.
	if p.Timeout >= 0 {
		t.Errorf("Timeout = %v, expected negative", p.Timeout)
	}
}
