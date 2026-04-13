package measure

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Darkroom4364/netlens/tomo"
)

// PerfSONARSource is an HTTP client for the esmond REST API (PerfSONAR).
type PerfSONARSource struct {
	BaseURL    string // e.g., "https://ps.example.com/esmond/perfsonar/archive"
	HTTPClient *http.Client
}

// NewPerfSONARSource creates a new PerfSONAR esmond API client.
// If client is nil, a default client with 30s timeout is used.
func NewPerfSONARSource(baseURL string, client *http.Client) *PerfSONARSource {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &PerfSONARSource{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		HTTPClient: client,
	}
}

// esmondMetadata represents one metadata record from the esmond archive.
type esmondMetadata struct {
	Source      string           `json:"source"`
	Destination string          `json:"destination"`
	EventTypes  []esmondEvtType `json:"event-types"`
}

// esmondEvtType describes an event-type entry inside metadata.
type esmondEvtType struct {
	EventType string `json:"event-type"`
	BaseURI   string `json:"base-uri"`
}

// esmondTSPoint is a single timeseries data point.
type esmondTSPoint struct {
	Timestamp int64   `json:"ts"`
	Value     float64 `json:"val"`
}

// FetchLatency fetches one-way latency timeseries between src and dst.
// It queries the esmond metadata endpoint, finds the packet-trace event type,
// then fetches the timeseries and maps each point to a PathMeasurement.
func (s *PerfSONARSource) FetchLatency(ctx context.Context, src, dst string, timeRangeSec int64) ([]tomo.PathMeasurement, error) {
	// Step 1: fetch metadata to find the base-uri for packet-trace.
	params := url.Values{}
	params.Set("source", src)
	params.Set("destination", dst)
	params.Set("event-type", "packet-trace")
	if timeRangeSec > 0 {
		params.Set("time-range", strconv.FormatInt(timeRangeSec, 10))
	}
	metaURL := s.BaseURL + "?" + params.Encode()

	metaBody, err := s.doGet(ctx, metaURL)
	if err != nil {
		return nil, fmt.Errorf("perfsonar metadata query: %w", err)
	}

	var records []esmondMetadata
	if err := json.Unmarshal(metaBody, &records); err != nil {
		return nil, fmt.Errorf("perfsonar parse metadata: %w", err)
	}
	if len(records) == 0 {
		return nil, nil
	}

	// Find the base-uri for packet-trace from the first matching record.
	var baseURI string
	for _, evt := range records[0].EventTypes {
		if evt.EventType == "packet-trace" {
			baseURI = evt.BaseURI
			break
		}
	}
	if baseURI == "" {
		return nil, fmt.Errorf("perfsonar: no packet-trace event-type for %s -> %s", src, dst)
	}

	// Step 2: fetch timeseries data from the base-uri.
	// The base-uri is a path; resolve it relative to the archive host.
	tsURL := s.resolveURI(baseURI)
	tsBody, err := s.doGet(ctx, tsURL)
	if err != nil {
		return nil, fmt.Errorf("perfsonar timeseries fetch: %w", err)
	}

	var points []esmondTSPoint
	if err := json.Unmarshal(tsBody, &points); err != nil {
		return nil, fmt.Errorf("perfsonar parse timeseries: %w", err)
	}

	// Step 3: map to PathMeasurements (end-to-end only, no hops).
	measurements := make([]tomo.PathMeasurement, 0, len(points))
	for _, pt := range points {
		measurements = append(measurements, tomo.PathMeasurement{
			Src:       records[0].Source,
			Dst:       records[0].Destination,
			RTTs:      []time.Duration{time.Duration(pt.Value * float64(time.Millisecond))},
			Timestamp: time.Unix(pt.Timestamp, 0),
			Weight:    1.0,
		})
	}
	return measurements, nil
}

// resolveURI turns an esmond base-uri path into an absolute URL
// by using the scheme and host from BaseURL.
func (s *PerfSONARSource) resolveURI(path string) string {
	parsed, err := url.Parse(s.BaseURL)
	if err != nil {
		return s.BaseURL + path
	}
	return parsed.Scheme + "://" + parsed.Host + path
}

// doGet performs a GET request with 429 Retry-After handling.
func (s *PerfSONARSource) doGet(ctx context.Context, rawURL string) ([]byte, error) {
	const maxRetries = 3

	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Accept", "application/json")

		resp, err := s.HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http get %s: %w", rawURL, err)
		}

		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read response body: %w", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
			if attempt == maxRetries {
				return nil, fmt.Errorf("rate limited after %d retries (GET %s)", maxRetries, rawURL)
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(retryAfter):
				continue
			}
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("HTTP %d from %s: %s", resp.StatusCode, rawURL, truncateBody(body, 200))
		}

		return body, nil
	}
	return nil, fmt.Errorf("exhausted retries for %s", rawURL)
}
