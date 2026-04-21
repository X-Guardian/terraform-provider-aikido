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

const usersPerPage = 100

// ListUsers returns all users for the given filters, served from a short-lived
// cache when possible so multiple resources sharing the same filters perform
// one paginated fetch.
func (c *AikidoClient) ListUsers(ctx context.Context, opts *ListUsersOptions) ([]User, error) {
	entry, err := c.usersCache.getOrFetch(ctx, usersCacheKey(opts), func() ([]User, error) {
		return c.listUsersUncached(ctx, opts)
	})
	if err != nil {
		return nil, err
	}
	return entry.users, nil
}

// listUsersUncached performs the actual paginated fetch, bypassing the cache.
func (c *AikidoClient) listUsersUncached(ctx context.Context, opts *ListUsersOptions) ([]User, error) {
	var allUsers []User
	err := c.iterateUsersPages(ctx, opts, func(users []User) (bool, error) {
		allUsers = append(allUsers, users...)
		return false, nil
	})
	if err != nil {
		return nil, err
	}
	return allUsers, nil
}

// iterateUsersPages walks pages sequentially, invoking fn on each page's results.
// If fn returns stop=true or an error, iteration ends.
func (c *AikidoClient) iterateUsersPages(ctx context.Context, opts *ListUsersOptions, fn func([]User) (bool, error)) error {
	page := 0
	for {
		params := url.Values{}
		params.Set("page", strconv.Itoa(page))
		params.Set("per_page", strconv.Itoa(usersPerPage))

		if opts != nil {
			if opts.TeamID != nil {
				params.Set("filter_team_id", strconv.Itoa(*opts.TeamID))
			}
			if opts.IncludeInactive {
				params.Set("include_inactive", "1")
			}
		}

		users, err := c.getUsersPage(ctx, params)
		if err != nil {
			return err
		}

		if len(users) == 0 {
			return nil
		}

		stop, err := fn(users)
		if err != nil {
			return err
		}
		if stop {
			return nil
		}

		if len(users) < usersPerPage {
			return nil
		}

		page++
	}
}

// getUsersPage fetches a single page of users.
func (c *AikidoClient) getUsersPage(ctx context.Context, params url.Values) ([]User, error) {
	resp, err := c.DoRequest(ctx, http.MethodGet, "/users?"+params.Encode(), nil)
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
// paginated API call rather than N.
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
