package client

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// AikidoClient handles authentication and HTTP requests to the Aikido API.
type AikidoClient struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	HTTPClient   *http.Client

	mu          sync.Mutex
	accessToken string
	tokenExpiry time.Time
	limiter     *rate.Limiter
	usersCache  *usersCache
	teamsCache  *teamsCache
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

// NewAikidoClient creates a new Aikido API client.
// The rate limiter is set to 18 requests per minute (slightly under the 20/min API limit)
// to avoid hitting 429s from parallel Terraform operations.
func NewAikidoClient(baseURL, clientID, clientSecret string) *AikidoClient {
	return &AikidoClient{
		BaseURL:      strings.TrimRight(baseURL, "/"),
		ClientID:     clientID,
		ClientSecret: clientSecret,
		HTTPClient:   &http.Client{Timeout: 30 * time.Second},
		limiter:      rate.NewLimiter(rate.Every(time.Minute/18), 1),
		usersCache:   newUsersCache(5 * time.Minute),
		teamsCache:   newTeamsCache(5 * time.Minute),
	}
}

// SetRateLimit overrides the default rate limiter. Useful for testing.
func (c *AikidoClient) SetRateLimit(requestsPerSecond float64) {
	c.limiter = rate.NewLimiter(rate.Limit(requestsPerSecond), 1)
}

// authenticate obtains or refreshes the OAuth2 access token.
func (c *AikidoClient) authenticate(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Token still valid — skip refresh.
	if c.accessToken != "" && time.Now().Before(c.tokenExpiry) {
		return nil
	}

	tokenURL := c.BaseURL + "/api/oauth/token"

	form := url.Values{}
	form.Set("grant_type", "client_credentials")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("creating token request: %w", err)
	}

	credentials := base64.StdEncoding.EncodeToString([]byte(c.ClientID + ":" + c.ClientSecret))
	req.Header.Set("Authorization", "Basic "+credentials)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("requesting token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("token request failed (status %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("decoding token response: %w", err)
	}

	c.accessToken = tokenResp.AccessToken
	// Refresh 60 seconds before actual expiry to avoid edge cases.
	c.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn-60) * time.Second)

	return nil
}

// DoRequest performs an authenticated HTTP request to the Aikido API.
// The path should be relative to /api/public/v1 (e.g., "/teams").
// Requests are rate-limited to stay within the API's 20 calls/minute limit.
// Automatically retries on 429 Too Many Requests using the Retry-After header.
func (c *AikidoClient) DoRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	if err := c.authenticate(ctx); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	fullURL := c.BaseURL + "/api/public/v1" + path

	var jsonBytes []byte
	if body != nil {
		var err error
		jsonBytes, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body: %w", err)
		}
	}

	maxRetries := 3
	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Wait for rate limiter before sending.
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limiter: %w", err)
		}

		var bodyReader io.Reader
		if jsonBytes != nil {
			bodyReader = bytes.NewReader(jsonBytes)
		}

		req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+c.accessToken)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("executing request: %w", err)
		}

		if resp.StatusCode != http.StatusTooManyRequests {
			return resp, nil
		}

		resp.Body.Close()

		if attempt == maxRetries {
			return nil, fmt.Errorf("rate limited after %d retries", maxRetries)
		}

		// Back off using Retry-After header, falling back to 30s.
		retryAfter := resp.Header.Get("Retry-After")
		wait := 30 * time.Second
		if retryAfter != "" {
			if seconds, err := strconv.Atoi(retryAfter); err == nil {
				wait = time.Duration(seconds) * time.Second
			}
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}

	return nil, fmt.Errorf("rate limited: exhausted retries")
}

// errorBody reads an HTTP response body and returns a clean error string.
// If the body is a JSON object with an "error" key, that value is returned.
// Otherwise the raw body is returned as-is.
func errorBody(body []byte) string {
	var parsed struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &parsed) == nil && parsed.Error != "" {
		return parsed.Error
	}
	return string(body)
}
