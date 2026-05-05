package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

// User represents a user in the Aikido API.
type User struct {
	ID                 int    `json:"id"`
	FullName           string `json:"full_name"`
	Email              string `json:"email"`
	Active             int    `json:"active"`
	LastLoginTimestamp int64  `json:"last_login_timestamp"`
	Role               string `json:"role"`
	AuthType           string `json:"auth_type"`
}

// ListUsersOptions contains optional filters for listing users.
type ListUsersOptions struct {
	TeamID          *int
	IncludeInactive bool
}

// ListUsers returns all users for the given filters, served from a short-lived
// cache when possible so multiple resources sharing the same filters perform
// one fetch.
func (c *AikidoClient) ListUsers(ctx context.Context, opts *ListUsersOptions) ([]User, error) {
	entry, err := c.usersCache.getOrFetch(ctx, usersCacheKey(opts), func() ([]User, error) {
		return c.listUsersUncached(ctx, opts)
	})
	if err != nil {
		return nil, err
	}
	return entry.users, nil
}

// listUsersUncached fetches the user list directly, bypassing the cache.
// The /users endpoint returns the full list in a single response (no pagination).
func (c *AikidoClient) listUsersUncached(ctx context.Context, opts *ListUsersOptions) ([]User, error) {
	params := url.Values{}
	if opts != nil {
		if opts.TeamID != nil {
			params.Set("filter_team_id", strconv.Itoa(*opts.TeamID))
		}
		if opts.IncludeInactive {
			params.Set("include_inactive", "1")
		}
	}

	path := "/users"
	if encoded := params.Encode(); encoded != "" {
		path += "?" + encoded
	}

	resp, err := c.DoRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d listing users: %s", resp.StatusCode, errorBody(body))
	}

	var users []User
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return nil, fmt.Errorf("decoding users response: %w", err)
	}

	return users, nil
}

// IsUserInTeam checks if a user is a member of a team. The team's user list is
// cached on first fetch so N membership checks against the same team cost one
// API call rather than N.
func (c *AikidoClient) IsUserInTeam(ctx context.Context, teamID, userID int) (bool, error) {
	opts := &ListUsersOptions{TeamID: &teamID}
	entry, err := c.usersCache.getOrFetch(ctx, usersCacheKey(opts), func() ([]User, error) {
		return c.listUsersUncached(ctx, opts)
	})
	if err != nil {
		return false, fmt.Errorf("listing users for team: %w", err)
	}
	_, found := entry.userIDs[userID]
	return found, nil
}
