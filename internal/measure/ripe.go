package measure

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/Darkroom4364/netlens/internal/tomo"
)

// Default RIPE Atlas API base URL.
const DefaultRIPEAtlasBaseURL = "https://atlas.ripe.net/api/v2"

// RIPEAtlasSource is an HTTP client for the RIPE Atlas v2 API.
type RIPEAtlasSource struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
}

// NewRIPEAtlasSource creates a new RIPE Atlas API client.
// If baseURL is empty, the default production URL is used.
// If httpClient is nil, http.DefaultClient is used.
func NewRIPEAtlasSource(apiKey, baseURL string, httpClient *http.Client) *RIPEAtlasSource {
	if baseURL == "" {
		baseURL = DefaultRIPEAtlasBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &RIPEAtlasSource{
		APIKey:     apiKey,
		BaseURL:    baseURL,
		HTTPClient: httpClient,
	}
}

// MeasurementStatus represents the status response from GET /measurements/{id}/.
type MeasurementStatus struct {
	ID          int    `json:"id"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Status      struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"status"`
	CreationTime int64 `json:"creation_time"`
	StartTime    int64 `json:"start_time"`
	StopTime     int64 `json:"stop_time"`
	IsOneoff     bool  `json:"is_oneoff"`
}

// IsStopped returns true if the measurement status indicates completion (status 4).
func (s *MeasurementStatus) IsStopped() bool {
	return s.Status.ID == 4
}

// IsOngoing returns true if the measurement is currently active (status 2).
func (s *MeasurementStatus) IsOngoing() bool {
	return s.Status.ID == 2
}

// SearchQuery holds parameters for searching RIPE Atlas measurements.
type SearchQuery struct {
	Type                string // e.g. "traceroute"
	Status              int    // e.g. 2 = Ongoing
	DescriptionContains string
	PageSize            int
}

// MeasurementInfo is a summary entry returned by the measurements list endpoint.
type MeasurementInfo struct {
	ID          int    `json:"id"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Status      struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"status"`
	CreationTime int64 `json:"creation_time"`
	StartTime    int64 `json:"start_time"`
	StopTime     int64 `json:"stop_time"`
	IsOneoff     bool  `json:"is_oneoff"`
}

// paginatedResponse is the envelope for paginated list endpoints.
type paginatedResponse struct {
	Count    int               `json:"count"`
	Next     string            `json:"next"`
	Previous string            `json:"previous"`
	Results  []json.RawMessage `json:"results"`
}

// MeasurementIter is a streaming iterator over parsed traceroute measurements.
type MeasurementIter interface {
	Next() bool
	Measurement() tomo.PathMeasurement
	Err() error
	Close() error
}

// sliceIter implements MeasurementIter over a pre-fetched slice.
type sliceIter struct {
	data []tomo.PathMeasurement
	idx  int
	err  error
}

func newSliceIter(data []tomo.PathMeasurement, err error) *sliceIter {
	return &sliceIter{data: data, idx: -1, err: err}
}

func (it *sliceIter) Next() bool {
	if it.err != nil {
		return false
	}
	it.idx++
	return it.idx < len(it.data)
}

func (it *sliceIter) Measurement() tomo.PathMeasurement {
	if it.idx < 0 || it.idx >= len(it.data) {
		return tomo.PathMeasurement{}
	}
	return it.data[it.idx]
}

func (it *sliceIter) Err() error  { return it.err }
func (it *sliceIter) Close() error { return nil }

// streamIter implements MeasurementIter with lazy page fetching.
type streamIter struct {
	src     *RIPEAtlasSource
	ctx     context.Context
	msmID   int
	start   int64
	stop    int64
	buf     []tomo.PathMeasurement
	idx     int
	fetched bool
	err     error
	mu      sync.Mutex
}

func (it *streamIter) fetch() {
	data, err := it.src.FetchResults(it.ctx, it.msmID, it.start, it.stop)
	it.buf = data
	it.err = err
	it.fetched = true
}

func (it *streamIter) Next() bool {
	it.mu.Lock()
	defer it.mu.Unlock()
	if !it.fetched {
		it.fetch()
	}
	if it.err != nil {
		return false
	}
	it.idx++
	return it.idx < len(it.buf)
}

func (it *streamIter) Measurement() tomo.PathMeasurement {
	it.mu.Lock()
	defer it.mu.Unlock()
	if it.idx < 0 || it.idx >= len(it.buf) {
		return tomo.PathMeasurement{}
	}
	return it.buf[it.idx]
}

func (it *streamIter) Err() error {
	it.mu.Lock()
	defer it.mu.Unlock()
	return it.err
}

func (it *streamIter) Close() error { return nil }

// Iter returns a streaming iterator over traceroute results for a measurement.
func (s *RIPEAtlasSource) Iter(ctx context.Context, msmID int, start, stop int64) MeasurementIter {
	return &streamIter{
		src:   s,
		ctx:   ctx,
		msmID: msmID,
		start: start,
		stop:  stop,
		idx:   -1,
	}
}

// FetchResults fetches traceroute results for a measurement and parses them
// into PathMeasurements using ParseRIPEAtlasTraceroute.
func (s *RIPEAtlasSource) FetchResults(ctx context.Context, msmID int, start, stop int64) ([]tomo.PathMeasurement, error) {
	u := fmt.Sprintf("%s/measurements/%d/results/", s.BaseURL, msmID)

	params := url.Values{}
	if start > 0 {
		params.Set("start", strconv.FormatInt(start, 10))
	}
	if stop > 0 {
		params.Set("stop", strconv.FormatInt(stop, 10))
	}
	if len(params) > 0 {
		u += "?" + params.Encode()
	}

	body, err := s.doGet(ctx, u)
	if err != nil {
		return nil, fmt.Errorf("fetch results for msm %d: %w", msmID, err)
	}

	return ParseRIPEAtlasTraceroute(body)
}

// FetchStatus retrieves the status of a measurement.
func (s *RIPEAtlasSource) FetchStatus(ctx context.Context, msmID int) (*MeasurementStatus, error) {
	u := fmt.Sprintf("%s/measurements/%d/", s.BaseURL, msmID)

	body, err := s.doGet(ctx, u)
	if err != nil {
		return nil, fmt.Errorf("fetch status for msm %d: %w", msmID, err)
	}

	var status MeasurementStatus
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, fmt.Errorf("parse status for msm %d: %w", msmID, err)
	}
	return &status, nil
}

// SearchMeasurements searches for measurements matching the given query.
// It follows pagination to collect all results.
func (s *RIPEAtlasSource) SearchMeasurements(ctx context.Context, query SearchQuery) ([]MeasurementInfo, error) {
	params := url.Values{}
	if query.Type != "" {
		params.Set("type", query.Type)
	}
	if query.Status > 0 {
		params.Set("status", strconv.Itoa(query.Status))
	}
	if query.DescriptionContains != "" {
		params.Set("description__contains", query.DescriptionContains)
	}
	if query.PageSize > 0 {
		params.Set("page_size", strconv.Itoa(query.PageSize))
	}

	u := fmt.Sprintf("%s/measurements/?%s", s.BaseURL, params.Encode())

	var all []MeasurementInfo
	for u != "" {
		body, err := s.doGet(ctx, u)
		if err != nil {
			return nil, fmt.Errorf("search measurements: %w", err)
		}

		var page paginatedResponse
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("parse search results: %w", err)
		}

		for _, raw := range page.Results {
			var info MeasurementInfo
			if err := json.Unmarshal(raw, &info); err != nil {
				return nil, fmt.Errorf("parse measurement info: %w", err)
			}
			all = append(all, info)
		}

		u = page.Next
	}
	return all, nil
}

// WaitForResults polls the measurement status until results are available
// or the timeout expires. Once the measurement is stopped (status 4) or
// ongoing (status 2), it fetches and returns the results.
func (s *RIPEAtlasSource) WaitForResults(ctx context.Context, msmID int, timeout time.Duration) ([]tomo.PathMeasurement, error) {
	deadline := time.Now().Add(timeout)
	pollInterval := 5 * time.Second

	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for results of msm %d", msmID)
		}

		status, err := s.FetchStatus(ctx, msmID)
		if err != nil {
			return nil, err
		}

		// Status 4 = Stopped (finished), Status 2 = Ongoing (has partial results)
		if status.IsStopped() || status.IsOngoing() {
			return s.FetchResults(ctx, msmID, 0, 0)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}

		// Exponential backoff capped at 30s
		if pollInterval < 30*time.Second {
			pollInterval = pollInterval * 2
			if pollInterval > 30*time.Second {
				pollInterval = 30 * time.Second
			}
		}
	}
}

// doGet performs an authenticated GET request with rate-limit handling.
func (s *RIPEAtlasSource) doGet(ctx context.Context, rawURL string) ([]byte, error) {
	const maxRetries = 3

	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}

		if s.APIKey != "" {
			req.Header.Set("Authorization", "Key "+s.APIKey)
		}
		req.Header.Set("Accept", "application/json")

		resp, err := s.HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http get %s: %w", rawURL, err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
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

// parseRetryAfter parses the Retry-After header value (seconds).
// Falls back to 5 seconds if the header is missing or unparseable.
func parseRetryAfter(val string) time.Duration {
	if val == "" {
		return 5 * time.Second
	}
	secs, err := strconv.Atoi(val)
	if err != nil || secs < 0 {
		return 5 * time.Second
	}
	if secs == 0 {
		return 100 * time.Millisecond
	}
	return time.Duration(secs) * time.Second
}

// truncateBody returns a string of at most n bytes from body, for error messages.
func truncateBody(body []byte, n int) string {
	if len(body) <= n {
		return string(body)
	}
	return string(body[:n]) + "..."
}
