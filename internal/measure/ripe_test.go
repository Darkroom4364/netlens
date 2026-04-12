package measure

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Darkroom4364/netlens/internal/tomo"
)

// Mock RIPE Atlas traceroute results (same shape as the real API).
var mockTracerouteResults = `[
  {
    "type": "traceroute",
    "msm_id": 12345,
    "probe_id": 1001,
    "src_addr": "10.0.0.1",
    "dst_addr": "8.8.8.8",
    "dst_name": "dns.google",
    "timestamp": 1700000000,
    "proto": "ICMP",
    "paris_id": 1,
    "result": [
      {
        "hop": 1,
        "result": [
          {"from": "10.0.0.1", "rtt": 1.5, "size": 68, "ttl": 255}
        ]
      },
      {
        "hop": 2,
        "result": [
          {"from": "172.16.0.1", "rtt": 5.2, "size": 68, "ttl": 253}
        ]
      },
      {
        "hop": 3,
        "result": [
          {"from": "8.8.8.8", "rtt": 12.3, "size": 68, "ttl": 56},
          {"from": "8.8.8.8", "rtt": 11.8, "size": 68, "ttl": 56},
          {"from": "8.8.8.8", "rtt": 13.1, "size": 68, "ttl": 56}
        ]
      }
    ],
    "lts": 42
  }
]`

var mockMeasurementStatus = `{
  "id": 12345,
  "description": "Test traceroute to dns.google",
  "type": "traceroute",
  "status": {"id": 4, "name": "Stopped"},
  "creation_time": 1699999000,
  "start_time": 1699999100,
  "stop_time": 1700001000,
  "is_oneoff": true
}`

var mockSearchResults = `{
  "count": 2,
  "next": null,
  "previous": null,
  "results": [
    {
      "id": 12345,
      "description": "Test traceroute 1",
      "type": "traceroute",
      "status": {"id": 2, "name": "Ongoing"},
      "creation_time": 1699999000,
      "start_time": 1699999100,
      "stop_time": 0,
      "is_oneoff": false
    },
    {
      "id": 12346,
      "description": "Test traceroute 2",
      "type": "traceroute",
      "status": {"id": 4, "name": "Stopped"},
      "creation_time": 1699998000,
      "start_time": 1699998100,
      "stop_time": 1700000000,
      "is_oneoff": true
    }
  ]
}`

func newMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify auth header format.
		auth := r.Header.Get("Authorization")
		if auth != "" && auth != "Key test-api-key" {
			t.Errorf("unexpected Authorization header: %q", auth)
		}

		switch {
		case r.URL.Path == "/measurements/12345/results/":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, mockTracerouteResults)

		case r.URL.Path == "/measurements/12345/":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, mockMeasurementStatus)

		case r.URL.Path == "/measurements/" && r.URL.RawQuery != "":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, mockSearchResults)

		case r.URL.Path == "/measurements/99999/results/":
			w.WriteHeader(http.StatusTooManyRequests)
			w.Header().Set("Retry-After", "1")
			fmt.Fprint(w, `{"error": "rate limited"}`)

		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `{"error": "not found: %s"}`, r.URL.Path)
		}
	}))
}

func TestFetchResults(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	src := NewRIPEAtlasSource("test-api-key", srv.URL, srv.Client())
	ctx := context.Background()

	results, err := src.FetchResults(ctx, 12345, 0, 0)
	if err != nil {
		t.Fatalf("FetchResults: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}

	m := results[0]
	if m.Src != "10.0.0.1" {
		t.Errorf("Src = %q, want 10.0.0.1", m.Src)
	}
	if m.Dst != "8.8.8.8" {
		t.Errorf("Dst = %q, want 8.8.8.8", m.Dst)
	}
	if len(m.Hops) != 3 {
		t.Fatalf("got %d hops, want 3", len(m.Hops))
	}
	if m.Hops[0].IP != "10.0.0.1" {
		t.Errorf("hop 1 IP = %q, want 10.0.0.1", m.Hops[0].IP)
	}
	if m.Hops[2].IP != "8.8.8.8" {
		t.Errorf("hop 3 IP = %q, want 8.8.8.8", m.Hops[2].IP)
	}

	// 3 RTT samples from last hop
	if len(m.RTTs) != 3 {
		t.Errorf("got %d RTTs, want 3", len(m.RTTs))
	}

	// MinRTT should be ~11.8ms
	minRTT := m.MinRTT()
	if minRTT < 11*time.Millisecond || minRTT > 12*time.Millisecond {
		t.Errorf("MinRTT = %v, want ~11.8ms", minRTT)
	}

	// Timestamp
	if m.Timestamp.Unix() != 1700000000 {
		t.Errorf("Timestamp = %v, want unix 1700000000", m.Timestamp)
	}
}

func TestFetchStatus(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	src := NewRIPEAtlasSource("test-api-key", srv.URL, srv.Client())
	ctx := context.Background()

	status, err := src.FetchStatus(ctx, 12345)
	if err != nil {
		t.Fatalf("FetchStatus: %v", err)
	}

	if status.ID != 12345 {
		t.Errorf("ID = %d, want 12345", status.ID)
	}
	if status.Description != "Test traceroute to dns.google" {
		t.Errorf("Description = %q", status.Description)
	}
	if !status.IsStopped() {
		t.Error("expected IsStopped() = true for status.id=4")
	}
	if status.IsOngoing() {
		t.Error("expected IsOngoing() = false for status.id=4")
	}
	if !status.IsOneoff {
		t.Error("expected IsOneoff = true")
	}
}

func TestSearchMeasurements(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	src := NewRIPEAtlasSource("test-api-key", srv.URL, srv.Client())
	ctx := context.Background()

	results, err := src.SearchMeasurements(ctx, SearchQuery{
		Type:                "traceroute",
		Status:              2,
		DescriptionContains: "Test",
	})
	if err != nil {
		t.Fatalf("SearchMeasurements: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}

	if results[0].ID != 12345 {
		t.Errorf("results[0].ID = %d, want 12345", results[0].ID)
	}
	if results[1].ID != 12346 {
		t.Errorf("results[1].ID = %d, want 12346", results[1].ID)
	}
	if results[0].Status.Name != "Ongoing" {
		t.Errorf("results[0].Status.Name = %q, want Ongoing", results[0].Status.Name)
	}
}

func TestIterator(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	src := NewRIPEAtlasSource("test-api-key", srv.URL, srv.Client())
	ctx := context.Background()

	iter := src.Iter(ctx, 12345, 0, 0)
	defer iter.Close()

	var count int
	for iter.Next() {
		m := iter.Measurement()
		if m.Src == "" {
			t.Error("got empty Src from iterator")
		}
		count++
	}
	if err := iter.Err(); err != nil {
		t.Fatalf("iter error: %v", err)
	}
	if count != 1 {
		t.Errorf("iterator yielded %d items, want 1", count)
	}
}

func TestAuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockMeasurementStatus)
	}))
	defer srv.Close()

	src := NewRIPEAtlasSource("my-secret-key", srv.URL, srv.Client())
	_, err := src.FetchStatus(context.Background(), 12345)
	if err != nil {
		t.Fatalf("FetchStatus: %v", err)
	}

	want := "Key my-secret-key"
	if gotAuth != want {
		t.Errorf("Authorization header = %q, want %q", gotAuth, want)
	}
}

func TestRateLimitRetry(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts <= 2 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"error":"rate limited"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockTracerouteResults)
	}))
	defer srv.Close()

	src := NewRIPEAtlasSource("test-api-key", srv.URL, srv.Client())
	results, err := src.FetchResults(context.Background(), 12345, 0, 0)
	if err != nil {
		t.Fatalf("FetchResults after retry: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("got %d results, want 1", len(results))
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

func TestHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"detail":"Not found."}`)
	}))
	defer srv.Close()

	src := NewRIPEAtlasSource("test-api-key", srv.URL, srv.Client())
	_, err := src.FetchResults(context.Background(), 99999, 0, 0)
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
	}{
		{"", 5 * time.Second},
		{"10", 10 * time.Second},
		{"0", 100 * time.Millisecond},
		{"-1", 5 * time.Second},
		{"abc", 5 * time.Second},
		{"3", 3 * time.Second},
	}
	for _, tc := range tests {
		got := parseRetryAfter(tc.input)
		if got != tc.want {
			t.Errorf("parseRetryAfter(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestPaginatedSearch(t *testing.T) {
	page := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		w.Header().Set("Content-Type", "application/json")

		item1, _ := json.Marshal(MeasurementInfo{ID: page*10 + 1, Description: "item1"})
		item2, _ := json.Marshal(MeasurementInfo{ID: page*10 + 2, Description: "item2"})

		var nextURL string
		if page == 1 {
			nextURL = fmt.Sprintf(`"%s/measurements/?page=2"`, r.Host)
			// Use the server URL for the next link.
			nextURL = fmt.Sprintf(`"http://%s/measurements/?page=2"`, r.Host)
		}

		resp := fmt.Sprintf(`{"count":4,"next":%s,"previous":null,"results":[%s,%s]}`,
			func() string {
				if page == 1 {
					return nextURL
				}
				return "null"
			}(),
			string(item1), string(item2))
		fmt.Fprint(w, resp)
	}))
	defer srv.Close()

	src := NewRIPEAtlasSource("test-api-key", srv.URL, srv.Client())
	results, err := src.SearchMeasurements(context.Background(), SearchQuery{Type: "traceroute"})
	if err != nil {
		t.Fatalf("SearchMeasurements: %v", err)
	}

	if len(results) != 4 {
		t.Errorf("got %d results, want 4 (2 pages x 2)", len(results))
	}
}

func TestSliceIter(t *testing.T) {
	data := []tomo.PathMeasurement{
		{Src: "a", Dst: "b"},
		{Src: "c", Dst: "d"},
	}
	iter := newSliceIter(data, nil)

	var got []string
	for iter.Next() {
		got = append(got, iter.Measurement().Src)
	}
	if err := iter.Err(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "c" {
		t.Errorf("got %v, want [a c]", got)
	}
}

func TestSliceIterError(t *testing.T) {
	iter := newSliceIter(nil, fmt.Errorf("test error"))
	if iter.Next() {
		t.Error("Next() should return false on error")
	}
	if iter.Err() == nil {
		t.Error("Err() should return the error")
	}
}
