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

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"golang.org/x/time/rate"
)

// RateLimitTier selects the rate at which requests are issued to the Aikido API.
// Each tier is set just under the corresponding API quota to give headroom for
// parallel Terraform operations.
type RateLimitTier int

const (
	// RateLimitTierStandard targets the default 20 req/min API quota.
	RateLimitTierStandard RateLimitTier = iota
	// RateLimitTierEnhanced targets the higher 50 req/min API quota.
	RateLimitTierEnhanced
)

const (
	// standardTierRPM matches the Aikido API quota for the standard tier.
	// We pace at the full quota; any overshoot from parallel ops is handled by
	// the Retry-After backoff in DoRequest.
	standardTierRPM = 20
	// enhancedTierRPM matches the Aikido API quota for the enhanced tier.
	// Same rationale as standardTierRPM.
	enhancedTierRPM = 50

	// rateLimiterBurst lets short clusters of requests fire immediately while the
	// long-run average stays bounded by the tier's RPM.
	rateLimiterBurst = 3

	// maxRateLimitRetries caps how many times a request retries after a 429 or transient 5xx.
	maxRateLimitRetries = 8
	// defaultRetryAfter is the base wait when the server returns 429 without a
	// Retry-After header. Subsequent attempts back off exponentially from this.
	defaultRetryAfter = 5 * time.Second
	// maxBackoff caps the per-attempt wait so retries stay bounded.
	maxBackoff = 60 * time.Second
	// transient5xxBackoff is the base wait between retries for 502/503/504 responses.
	transient5xxBackoff = 5 * time.Second
)

// RateLimitTierFromString parses a tier name from config or env. Empty or
// unknown values fall back to the standard tier.
func RateLimitTierFromString(s string) RateLimitTier {
	switch s {
	case "enhanced":
		return RateLimitTierEnhanced
	default:
		return RateLimitTierStandard
	}
}

func newLimiterForTier(tier RateLimitTier) *rate.Limiter {
	rpm := standardTierRPM
	if tier == RateLimitTierEnhanced {
		rpm = enhancedTierRPM
	}
	return rate.NewLimiter(rate.Every(time.Minute/time.Duration(rpm)), rateLimiterBurst)
}

// AikidoClient handles authentication and HTTP requests to the Aikido API.
type AikidoClient struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	HTTPClient   *http.Client

	mu              sync.Mutex
	accessToken     string
	tokenExpiry     time.Time
	limiter         *rate.Limiter
	usersCache      *usersCache
	teamsCache      *teamsCache
	containersCache *containersCache
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

// NewAikidoClient creates a new Aikido API client at the given rate limit tier.
func NewAikidoClient(baseURL, clientID, clientSecret string, tier RateLimitTier) *AikidoClient {
	return &AikidoClient{
		BaseURL:      strings.TrimRight(baseURL, "/"),
		ClientID:     clientID,
		ClientSecret: clientSecret,
		HTTPClient:   &http.Client{Timeout: 30 * time.Second},
		limiter:      newLimiterForTier(tier),
		usersCache:   newUsersCache(5 * time.Minute),
		teamsCache:   newTeamsCache(5 * time.Minute),
		// Containers use a shorter TTL than the other caches because the same values are also
		// written by this provider. Every write invalidates the cache, so the TTL only bounds how
		// stale an out-of-band change can be within a single plan or apply.
		containersCache: newContainersCache(30 * time.Second),
	}
}

// SetRateLimitTier replaces the client's rate limiter with one for the given tier.
func (c *AikidoClient) SetRateLimitTier(tier RateLimitTier) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.limiter = newLimiterForTier(tier)
}

// SetRateLimitForTesting replaces the rate limiter with one that allows the given
// requests-per-second. Use this only from tests to avoid pacing delays.
func (c *AikidoClient) SetRateLimitForTesting(requestsPerSecond float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.limiter = rate.NewLimiter(rate.Limit(requestsPerSecond), rateLimiterBurst)
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
// Requests are rate-limited per the configured tier and automatically retried
// on 429 Too Many Requests (using the Retry-After header) and transient 5xx errors.
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

	for attempt := 0; attempt <= maxRateLimitRetries; attempt++ {
		// Wait for rate limiter before sending. Snapshot under the mutex so a
		// concurrent SetRateLimitTier call cannot race the pointer read.
		c.mu.Lock()
		limiter := c.limiter
		c.mu.Unlock()
		if err := limiter.Wait(ctx); err != nil {
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

		if !shouldRetryStatus(resp.StatusCode) {
			return resp, nil
		}

		wait := backoffForResponse(resp, attempt)
		resp.Body.Close()

		if attempt == maxRateLimitRetries {
			return nil, fmt.Errorf("request to %s failed with status %d after %d retries", path, resp.StatusCode, maxRateLimitRetries)
		}

		tflog.Debug(ctx, "retrying aikido request after retryable status", map[string]interface{}{
			"method":  method,
			"path":    path,
			"status":  resp.StatusCode,
			"attempt": attempt + 1,
			"max":     maxRateLimitRetries,
			"wait":    wait.String(),
		})

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}

	return nil, fmt.Errorf("request to %s exhausted retries", path)
}

// shouldRetryStatus reports whether a status code is one we retry transparently.
func shouldRetryStatus(status int) bool {
	switch status {
	case http.StatusTooManyRequests,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	}
	return false
}

// backoffForResponse picks a wait duration based on the response and the
// zero-based attempt number. A 429 with a Retry-After header honours it exactly;
// otherwise the wait grows exponentially from the base (per status) with a small
// deterministic jitter, capped at maxBackoff. Exponential backoff spreads out the
// many concurrent retries a large Terraform plan generates so they stop colliding.
func backoffForResponse(resp *http.Response, attempt int) time.Duration {
	if resp.StatusCode == http.StatusTooManyRequests {
		if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
			if seconds, err := strconv.Atoi(retryAfter); err == nil {
				return time.Duration(seconds) * time.Second
			}
		}
		return expBackoff(defaultRetryAfter, attempt)
	}
	return expBackoff(transient5xxBackoff, attempt)
}

// expBackoff returns base * 2^attempt plus deterministic jitter, capped at maxBackoff.
// Jitter is derived from the attempt number (not a global RNG) so behaviour stays
// reproducible in tests while still de-synchronising concurrent retries by a fraction.
func expBackoff(base time.Duration, attempt int) time.Duration {
	wait := base << attempt
	if wait <= 0 || wait > maxBackoff {
		wait = maxBackoff
	}
	// Jitter up to ~12.5% of the wait, varied per attempt.
	jitter := (wait / 8) * time.Duration(attempt%3) / 2
	wait += jitter
	if wait > maxBackoff {
		wait = maxBackoff
	}
	return wait
}

// errorBody reads an HTTP response body and returns a clean error string.
// If the body is a JSON object with an "error" or "reason_phrase" key, that value is
// returned, preferring "error" when both are present.
// Otherwise the raw body is returned as-is.
func errorBody(body []byte) string {
	var parsed struct {
		Error        string `json:"error"`
		ReasonPhrase string `json:"reason_phrase"`
	}
	if json.Unmarshal(body, &parsed) == nil {
		if parsed.Error != "" {
			return parsed.Error
		}
		if parsed.ReasonPhrase != "" {
			return parsed.ReasonPhrase
		}
	}
	return string(body)
}
