package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ZenApp represents a Zen app in the list response.
type ZenApp struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	CreatedAt    int64   `json:"created_at"`
	CodeRepoID   int     `json:"code_repo_id"`
	CodeRepoName *string `json:"code_repo_name"`
	HasToken     bool    `json:"has_token"`
	TokenHint    string  `json:"token_hint"`
	Environment  string  `json:"environment"`
	Blocking     bool    `json:"blocking"`
}

// ZenAppDetail represents the detailed response from GET /firewall/apps/{id}.
type ZenAppDetail struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	CreatedAt       int64   `json:"created_at"`
	AmountInstances int     `json:"amount_instances"`
	CodeRepoID      int     `json:"code_repo_id"`
	CodeRepoName    *string `json:"code_repo_name"`
	HasToken        bool    `json:"has_token"`
	TokenHint       string  `json:"token_hint"`
	Environment     string  `json:"environment"`
}

// CreateZenAppRequest is the request body for creating a Zen app.
type CreateZenAppRequest struct {
	Name        string `json:"name"`
	Environment string `json:"environment"`
	RepoID      *int   `json:"repo_id,omitempty"`
}

// CreateZenAppResponse is the response from creating a Zen app.
type CreateZenAppResponse struct {
	AppID int    `json:"app_id"`
	Token string `json:"token"`
}

// UpdateZenAppRequest is the request body for updating a Zen app.
type UpdateZenAppRequest struct {
	Name        string `json:"name"`
	Environment string `json:"environment"`
}

// CreateZenApp creates a new Zen app and returns the app ID and token.
func (c *AikidoClient) CreateZenApp(ctx context.Context, req CreateZenAppRequest) (*CreateZenAppResponse, error) {
	resp, err := c.DoRequest(ctx, http.MethodPost, "/firewall/apps", req)
	if err != nil {
		return nil, fmt.Errorf("creating zen app: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d creating zen app: %s", resp.StatusCode, string(body))
	}

	var createResp CreateZenAppResponse
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		return nil, fmt.Errorf("decoding create zen app response: %w", err)
	}

	return &createResp, nil
}

// GetZenApp retrieves a single Zen app by ID.
func (c *AikidoClient) GetZenApp(ctx context.Context, appID int) (*ZenAppDetail, error) {
	resp, err := c.DoRequest(ctx, http.MethodGet, fmt.Sprintf("/firewall/apps/%d", appID), nil)
	if err != nil {
		return nil, fmt.Errorf("getting zen app: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("zen app with ID %d not found", appID)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d getting zen app: %s", resp.StatusCode, string(body))
	}

	var app ZenAppDetail
	if err := json.NewDecoder(resp.Body).Decode(&app); err != nil {
		return nil, fmt.Errorf("decoding zen app response: %w", err)
	}

	return &app, nil
}

// ListZenApps returns all Zen apps.
func (c *AikidoClient) ListZenApps(ctx context.Context) ([]ZenApp, error) {
	resp, err := c.DoRequest(ctx, http.MethodGet, "/firewall/apps", nil)
	if err != nil {
		return nil, fmt.Errorf("listing zen apps: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d listing zen apps: %s", resp.StatusCode, string(body))
	}

	var apps []ZenApp
	if err := json.NewDecoder(resp.Body).Decode(&apps); err != nil {
		return nil, fmt.Errorf("decoding zen apps response: %w", err)
	}

	return apps, nil
}

// UpdateZenApp updates an existing Zen app's name and environment.
func (c *AikidoClient) UpdateZenApp(ctx context.Context, appID int, req UpdateZenAppRequest) error {
	resp, err := c.DoRequest(ctx, http.MethodPut, fmt.Sprintf("/firewall/apps/%d", appID), req)
	if err != nil {
		return fmt.Errorf("updating zen app: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d updating zen app: %s", resp.StatusCode, string(body))
	}

	return nil
}

// DeleteZenApp deletes a Zen app by ID.
func (c *AikidoClient) DeleteZenApp(ctx context.Context, appID int) error {
	resp, err := c.DoRequest(ctx, http.MethodDelete, fmt.Sprintf("/firewall/apps/%d", appID), nil)
	if err != nil {
		return fmt.Errorf("deleting zen app: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d deleting zen app: %s", resp.StatusCode, string(body))
	}

	return nil
}
