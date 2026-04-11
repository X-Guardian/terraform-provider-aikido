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

// ListUsers returns all users by paginating through every page.
func (c *AikidoClient) ListUsers(ctx context.Context, opts *ListUsersOptions) ([]User, error) {
	var allUsers []User
	page := 0
	perPage := 20

	for {
		params := url.Values{}
		params.Set("page", strconv.Itoa(page))
		params.Set("per_page", strconv.Itoa(perPage))

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
			return nil, err
		}

		if len(users) == 0 {
			break
		}

		allUsers = append(allUsers, users...)

		if len(users) < perPage {
			break
		}

		page++
	}

	return allUsers, nil
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
