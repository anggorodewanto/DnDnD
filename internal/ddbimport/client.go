package ddbimport

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultBaseURL = "https://character-service.dndbeyond.com"

// Client fetches character data from D&D Beyond.
type Client interface {
	FetchCharacter(ctx context.Context, characterID string) ([]byte, error)
}

// DDBClient implements Client with exponential backoff for rate limiting.
type DDBClient struct {
	baseURL      string
	httpClient   *http.Client
	initialDelay time.Duration
	maxDelay     time.Duration
	maxRetries   int
}

// ClientOption configures a DDBClient.
type ClientOption func(*DDBClient)

// WithBaseURL sets the base URL for the DDB API.
func WithBaseURL(url string) ClientOption {
	return func(c *DDBClient) {
		c.baseURL = url
	}
}

// WithBackoff configures the exponential backoff parameters.
func WithBackoff(initial, max time.Duration, retries int) ClientOption {
	return func(c *DDBClient) {
		c.initialDelay = initial
		c.maxDelay = max
		c.maxRetries = retries
	}
}

// NewDDBClient creates a new DDB API client.
func NewDDBClient(opts ...ClientOption) *DDBClient {
	c := &DDBClient{
		baseURL:      defaultBaseURL,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		initialDelay: 1 * time.Second,
		maxDelay:     30 * time.Second,
		maxRetries:   3,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// FetchCharacter fetches a character's JSON data from DDB.
func (c *DDBClient) FetchCharacter(ctx context.Context, characterID string) ([]byte, error) {
	url := fmt.Sprintf("%s/character/v5/character/%s", c.baseURL, characterID)

	delay := c.initialDelay
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetching character: %w", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			if attempt == c.maxRetries {
				break
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
			delay *= 2
			if delay > c.maxDelay {
				delay = c.maxDelay
			}
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("reading response body: %w", err)
		}

		if resp.StatusCode == http.StatusForbidden {
			return nil, fmt.Errorf("HTTP 403: character may not be set to public sharing on D&D Beyond")
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
		}

		return body, nil
	}

	return nil, fmt.Errorf("rate limit exceeded after %d retries", c.maxRetries+1)
}
