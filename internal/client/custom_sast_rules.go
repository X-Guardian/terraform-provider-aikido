package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// CustomRule represents a custom SAST rule in the Aikido API.
type CustomRule struct {
	ID          int    `json:"id"`
	SemgrepRule string `json:"semgrep_rule"`
	IssueTitle  string `json:"issue_title"`
	TLDR        string `json:"tldr"`
	HowToFix    string `json:"how_to_fix"`
	Priority    int    `json:"priority"`
	Language    string `json:"language"`
	HasError    bool   `json:"has_error"`
}

// CustomRuleRequest is the request body for creating or updating a custom rule.
type CustomRuleRequest struct {
	SemgrepRule string `json:"semgrep_rule"`
	IssueTitle  string `json:"issue_title"`
	TLDR        string `json:"tldr"`
	HowToFix    string `json:"how_to_fix"`
	Priority    int    `json:"priority"`
	Language    string `json:"language"`
}

// CreateCustomRule creates a new custom SAST rule and returns its ID.
func (c *AikidoClient) CreateCustomRule(ctx context.Context, req CustomRuleRequest) (int, error) {
	resp, err := c.DoRequest(ctx, http.MethodPost, "/repositories/sast/custom-rules", req)
	if err != nil {
		return 0, fmt.Errorf("creating custom rule: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("unexpected status %d creating custom rule: %s", resp.StatusCode, string(body))
	}

	var createResp struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		return 0, fmt.Errorf("decoding create custom rule response: %w", err)
	}

	return createResp.ID, nil
}

// GetCustomRule retrieves a single custom rule by ID.
func (c *AikidoClient) GetCustomRule(ctx context.Context, ruleID int) (*CustomRule, error) {
	resp, err := c.DoRequest(ctx, http.MethodGet, fmt.Sprintf("/repositories/sast/custom-rules/%d", ruleID), nil)
	if err != nil {
		return nil, fmt.Errorf("getting custom rule: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("custom rule with ID %d not found", ruleID)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d getting custom rule: %s", resp.StatusCode, string(body))
	}

	var wrapper struct {
		CustomRule CustomRule `json:"custom_rule"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return nil, fmt.Errorf("decoding custom rule response: %w", err)
	}

	return &wrapper.CustomRule, nil
}

// ListCustomRules returns all custom SAST rules.
func (c *AikidoClient) ListCustomRules(ctx context.Context) ([]CustomRule, error) {
	resp, err := c.DoRequest(ctx, http.MethodGet, "/repositories/sast/custom-rules", nil)
	if err != nil {
		return nil, fmt.Errorf("listing custom rules: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d listing custom rules: %s", resp.StatusCode, string(body))
	}

	var wrapper struct {
		CustomRules []CustomRule `json:"custom_rules"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return nil, fmt.Errorf("decoding custom rules response: %w", err)
	}

	return wrapper.CustomRules, nil
}

// UpdateCustomRule updates an existing custom SAST rule.
func (c *AikidoClient) UpdateCustomRule(ctx context.Context, ruleID int, req CustomRuleRequest) error {
	resp, err := c.DoRequest(ctx, http.MethodPut, fmt.Sprintf("/repositories/sast/custom-rules/%d", ruleID), req)
	if err != nil {
		return fmt.Errorf("updating custom rule: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d updating custom rule: %s", resp.StatusCode, string(body))
	}

	return nil
}

// DeleteCustomRule deletes a custom SAST rule by ID.
func (c *AikidoClient) DeleteCustomRule(ctx context.Context, ruleID int) error {
	resp, err := c.DoRequest(ctx, http.MethodDelete, fmt.Sprintf("/repositories/sast/custom-rules/%d", ruleID), nil)
	if err != nil {
		return fmt.Errorf("deleting custom rule: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d deleting custom rule: %s", resp.StatusCode, string(body))
	}

	return nil
}
