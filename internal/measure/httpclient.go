package measure

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// RetryableClient wraps an HTTP client with rate-limit retry logic.
type RetryableClient struct {
	Client     *http.Client
	MaxRetries int // default 3
}

// Get performs an HTTP GET with automatic 429 retry.
// authHeader is optional (e.g. "Key xxx" for RIPE Atlas, "" for PerfSONAR).
func (c *RetryableClient) Get(ctx context.Context, url string, authHeader string) ([]byte, error) {
	for attempt := 0; attempt <= c.MaxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}
		req.Header.Set("Accept", "application/json")

		resp, err := c.Client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http get %s: %w", url, err)
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read response body: %w", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			if attempt == c.MaxRetries {
				return nil, fmt.Errorf("rate limited after %d retries (GET %s)", c.MaxRetries, url)
			}
			wait := parseRetryAfterHeader(resp.Header.Get("Retry-After"))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
				continue
			}
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("HTTP %d from %s: %s", resp.StatusCode, url, truncateBody(body, 200))
		}
		return body, nil
	}
	return nil, fmt.Errorf("exhausted retries for %s", url)
}

// parseRetryAfterHeader parses the Retry-After header value (seconds).
// Falls back to 5 seconds if the header is missing or unparseable.
func parseRetryAfterHeader(val string) time.Duration {
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
