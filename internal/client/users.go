// Copyright IBM Corp. 2025
// SPDX-License-Identifier: MPL-2.0

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
	ID                int    `json:"id"`
	FullName          string `json:"full_name"`
	Email             string `json:"email"`
	Active            int    `json:"active"`
	LastLoginTimestamp int64  `json:"last_login_timestamp"`
	Role              string `json:"role"`
	AuthType          string `json:"auth_type"`
}

// ListUsersOptions contains optional filters for listing users.
type ListUsersOptions struct {
	TeamID          *int
	IncludeInactive bool
}

// ListUsers returns all users, optionally filtered.
func (c *AikidoClient) ListUsers(ctx context.Context, opts *ListUsersOptions) ([]User, error) {
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
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	resp, err := c.DoRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d listing users: %s", resp.StatusCode, string(body))
	}

	var users []User
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return nil, fmt.Errorf("decoding users response: %w", err)
	}

	return users, nil
}

// IsUserInTeam checks if a user is a member of a team by listing users filtered by team.
func (c *AikidoClient) IsUserInTeam(ctx context.Context, teamID, userID int) (bool, error) {
	users, err := c.ListUsers(ctx, &ListUsersOptions{TeamID: &teamID})
	if err != nil {
		return false, fmt.Errorf("listing users for team: %w", err)
	}

	for _, u := range users {
		if u.ID == userID {
			return true, nil
		}
	}

	return false, nil
}
