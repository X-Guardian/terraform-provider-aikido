// Copyright IBM Corp. 2025
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Team represents a team in the Aikido API.
type Team struct {
	ID               int              `json:"id"`
	Name             string           `json:"name"`
	ExternalSource   *string          `json:"external_source"`
	ExternalSourceID *string          `json:"external_source_id"`
	Responsibilities []Responsibility `json:"responsibilities"`
	Active           bool             `json:"active"`
}

// Responsibility represents a resource assigned to a team.
type Responsibility struct {
	ID            int      `json:"id"`
	Type          string   `json:"type"`
	IncludedPaths []string `json:"included_paths"`
	ExcludedPaths []string `json:"excluded_paths"`
}

// CreateTeamRequest is the request body for creating a team.
type CreateTeamRequest struct {
	Name string `json:"name"`
}

// CreateTeamResponse is the response body from creating a team.
type CreateTeamResponse struct {
	ID int `json:"id"`
}

// UpdateTeamRequest is the request body for updating a team.
type UpdateTeamRequest struct {
	Name string `json:"name,omitempty"`
}

// CreateTeam creates a new team and returns the created team.
func (c *AikidoClient) CreateTeam(ctx context.Context, name string) (*Team, error) {
	resp, err := c.DoRequest(ctx, http.MethodPost, "/teams", CreateTeamRequest{Name: name})
	if err != nil {
		return nil, fmt.Errorf("creating team: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d creating team: %s", resp.StatusCode, string(body))
	}

	var createResp CreateTeamResponse
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		return nil, fmt.Errorf("decoding create team response: %w", err)
	}

	return c.GetTeam(ctx, createResp.ID)
}

// GetTeam retrieves a single team by ID by paginating through the list endpoint.
func (c *AikidoClient) GetTeam(ctx context.Context, teamID int) (*Team, error) {
	page := 0
	for {
		teams, err := c.getTeamsPage(ctx, page)
		if err != nil {
			return nil, err
		}

		if len(teams) == 0 {
			return nil, fmt.Errorf("team with ID %d not found", teamID)
		}

		for i := range teams {
			if teams[i].ID == teamID {
				return &teams[i], nil
			}
		}

		page++
	}
}

// ListTeams returns all teams by paginating through every page.
func (c *AikidoClient) ListTeams(ctx context.Context) ([]Team, error) {
	var allTeams []Team
	page := 0

	for {
		teams, err := c.getTeamsPage(ctx, page)
		if err != nil {
			return nil, err
		}

		if len(teams) == 0 {
			break
		}

		allTeams = append(allTeams, teams...)

		if len(teams) < 20 {
			break
		}

		page++
	}

	return allTeams, nil
}

// getTeamsPage fetches a single page of teams and closes the response body.
func (c *AikidoClient) getTeamsPage(ctx context.Context, page int) ([]Team, error) {
	resp, err := c.DoRequest(ctx, http.MethodGet, fmt.Sprintf("/teams?page=%d&per_page=20", page), nil)
	if err != nil {
		return nil, fmt.Errorf("listing teams (page %d): %w", page, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d listing teams: %s", resp.StatusCode, string(body))
	}

	var teams []Team
	if err := json.NewDecoder(resp.Body).Decode(&teams); err != nil {
		return nil, fmt.Errorf("decoding teams response: %w", err)
	}

	return teams, nil
}

// UpdateTeam updates an existing team.
func (c *AikidoClient) UpdateTeam(ctx context.Context, teamID int, req UpdateTeamRequest) error {
	resp, err := c.DoRequest(ctx, http.MethodPut, fmt.Sprintf("/teams/%d", teamID), req)
	if err != nil {
		return fmt.Errorf("updating team: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d updating team: %s", resp.StatusCode, string(body))
	}

	return nil
}

// resourceTypeToBodyField maps Aikido responsibility types to the API body field names.
var resourceTypeToBodyField = map[string]string{
	"code_repository":      "repo_id",
	"container_repository": "image_id",
	"cloud":                "cloud_id",
	"domain":               "domain_id",
	"zen_app":              "zen_app_id",
}

// LinkResourceToTeam links a resource to a team.
func (c *AikidoClient) LinkResourceToTeam(ctx context.Context, teamID int, resourceType string, resourceID int) error {
	field, ok := resourceTypeToBodyField[resourceType]
	if !ok {
		return fmt.Errorf("unknown resource type: %s", resourceType)
	}

	resp, err := c.DoRequest(ctx, http.MethodPost, fmt.Sprintf("/teams/%d/linkResource", teamID), map[string]int{field: resourceID})
	if err != nil {
		return fmt.Errorf("linking resource to team: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d linking resource to team: %s", resp.StatusCode, string(body))
	}

	return nil
}

// UnlinkResourceFromTeam unlinks a resource from a team.
func (c *AikidoClient) UnlinkResourceFromTeam(ctx context.Context, teamID int, resourceType string, resourceID int) error {
	field, ok := resourceTypeToBodyField[resourceType]
	if !ok {
		return fmt.Errorf("unknown resource type: %s", resourceType)
	}

	resp, err := c.DoRequest(ctx, http.MethodPost, fmt.Sprintf("/teams/%d/unlinkResource", teamID), map[string]int{field: resourceID})
	if err != nil {
		return fmt.Errorf("unlinking resource from team: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d unlinking resource from team: %s", resp.StatusCode, string(body))
	}

	return nil
}

// IsResourceLinkedToTeam checks if a resource is linked to a team via the team's responsibilities.
func (c *AikidoClient) IsResourceLinkedToTeam(ctx context.Context, teamID int, resourceType string, resourceID int) (bool, error) {
	team, err := c.GetTeam(ctx, teamID)
	if err != nil {
		return false, err
	}

	for _, r := range team.Responsibilities {
		if r.Type == resourceType && r.ID == resourceID {
			return true, nil
		}
	}

	return false, nil
}

// AddUserToTeam adds a user to a team.
func (c *AikidoClient) AddUserToTeam(ctx context.Context, teamID, userID int) error {
	resp, err := c.DoRequest(ctx, http.MethodPost, fmt.Sprintf("/teams/%d/addUser", teamID), map[string]int{"user_id": userID})
	if err != nil {
		return fmt.Errorf("adding user to team: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d adding user to team: %s", resp.StatusCode, string(body))
	}

	return nil
}

// RemoveUserFromTeam removes a user from a team.
func (c *AikidoClient) RemoveUserFromTeam(ctx context.Context, teamID, userID int) error {
	resp, err := c.DoRequest(ctx, http.MethodPost, fmt.Sprintf("/teams/%d/removeUser", teamID), map[string]int{"user_id": userID})
	if err != nil {
		return fmt.Errorf("removing user from team: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d removing user from team: %s", resp.StatusCode, string(body))
	}

	return nil
}

// DeleteTeam deletes a team by ID.
func (c *AikidoClient) DeleteTeam(ctx context.Context, teamID int) error {
	resp, err := c.DoRequest(ctx, http.MethodDelete, fmt.Sprintf("/teams/%d", teamID), nil)
	if err != nil {
		return fmt.Errorf("deleting team: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("cannot delete team (it may be an imported team): %s", string(body))
	}

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d deleting team: %s", resp.StatusCode, string(body))
	}

	return nil
}
