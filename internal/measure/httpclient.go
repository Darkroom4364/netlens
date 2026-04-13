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
			wait := time.Duration(ParseRetryAfter(resp)) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
				continue
			}
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("HTTP %d from %s: %s", resp.StatusCode, url, TruncateBody(string(body), 200))
		}
		return body, nil
	}
	return nil, fmt.Errorf("exhausted retries for %s", url)
}

// ParseRetryAfter extracts seconds from Retry-After header. Returns 1 on parse failure.
func ParseRetryAfter(resp *http.Response) int {
	val := resp.Header.Get("Retry-After")
	if val == "" {
		return 1
	}
	secs, err := strconv.Atoi(val)
	if err != nil || secs <= 0 {
		return 1
	}
	return secs
}

// TruncateBody returns the first maxLen bytes of body for error messages.
func TruncateBody(body string, maxLen int) string {
	if len(body) <= maxLen {
		return body
	}
	return body[:maxLen] + "..."
}
