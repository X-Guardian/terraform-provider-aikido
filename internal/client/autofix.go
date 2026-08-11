package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Autofix settings endpoint paths. These are workspace-wide singletons, so they take no
// path parameters.
const (
	autofixDependencySettingsPath = "/repositories/autofix/dependency/settings"
	autofixSastSettingsPath       = "/repositories/autofix/sast/settings"
	autofixPentestSettingsPath    = "/repositories/autofix/pentest/settings"
)

// ErrAutofixDisabledForWorkspace is returned when AutoFix is switched off for the whole
// Aikido workspace, which makes every autofix settings endpoint return 400.
var ErrAutofixDisabledForWorkspace = errors.New("autofix is disabled for this workspace")

// AutofixSettingsRequest is the PUT body for the dependency and SAST autofix settings
// endpoints. When Enabled is false the API ignores every other field and allows them to
// be omitted, so the remaining fields are pointers and are left out of the payload in
// that case.
type AutofixSettingsRequest struct {
	Enabled                  bool     `json:"enabled"`
	SeverityFilter           *string  `json:"severity_filter,omitempty"`
	ReposScope               *string  `json:"repos_scope,omitempty"`
	RepoIDs                  *[]int64 `json:"repo_ids,omitempty"`
	UseAikidoLibraryForMajor *bool    `json:"use_aikido_library_for_major,omitempty"`
}

// AutofixPentestSettingsRequest is the PUT body for the pentest autofix settings
// endpoint, which has no repository scoping.
type AutofixPentestSettingsRequest struct {
	Enabled        bool    `json:"enabled"`
	SeverityFilter *string `json:"severity_filter,omitempty"`
}

// AutofixDependencySettings is the workspace dependency autofix configuration.
type AutofixDependencySettings struct {
	Enabled                  bool    `json:"enabled"`
	SeverityFilter           string  `json:"severity_filter"`
	ReposScope               string  `json:"repos_scope"`
	RepoIDs                  []int64 `json:"repo_ids"`
	UseAikidoLibraryForMajor bool    `json:"use_aikido_library_for_major"`
}

// AutofixSastSettings is the workspace SAST and IaC autofix configuration.
type AutofixSastSettings struct {
	Enabled        bool    `json:"enabled"`
	SeverityFilter string  `json:"severity_filter"`
	ReposScope     string  `json:"repos_scope"`
	RepoIDs        []int64 `json:"repo_ids"`
}

// AutofixPentestSettings is the workspace pentest and AI code analysis autofix
// configuration.
type AutofixPentestSettings struct {
	Enabled        bool   `json:"enabled"`
	SeverityFilter string `json:"severity_filter"`
}

// GetAutofixDependencySettings retrieves the workspace dependency autofix settings.
func (c *AikidoClient) GetAutofixDependencySettings(ctx context.Context) (*AutofixDependencySettings, error) {
	var wrapper struct {
		Settings AutofixDependencySettings `json:"settings"`
	}
	if err := c.getAutofixSettings(ctx, autofixDependencySettingsPath, &wrapper); err != nil {
		return nil, err
	}
	return &wrapper.Settings, nil
}

// UpdateAutofixDependencySettings updates the workspace dependency autofix settings.
func (c *AikidoClient) UpdateAutofixDependencySettings(ctx context.Context, req AutofixSettingsRequest) error {
	return c.putAutofixSettings(ctx, autofixDependencySettingsPath, req)
}

// GetAutofixSastSettings retrieves the workspace SAST and IaC autofix settings.
func (c *AikidoClient) GetAutofixSastSettings(ctx context.Context) (*AutofixSastSettings, error) {
	var wrapper struct {
		Settings AutofixSastSettings `json:"settings"`
	}
	if err := c.getAutofixSettings(ctx, autofixSastSettingsPath, &wrapper); err != nil {
		return nil, err
	}
	return &wrapper.Settings, nil
}

// UpdateAutofixSastSettings updates the workspace SAST and IaC autofix settings.
func (c *AikidoClient) UpdateAutofixSastSettings(ctx context.Context, req AutofixSettingsRequest) error {
	return c.putAutofixSettings(ctx, autofixSastSettingsPath, req)
}

// GetAutofixPentestSettings retrieves the workspace pentest and AI code analysis autofix
// settings.
func (c *AikidoClient) GetAutofixPentestSettings(ctx context.Context) (*AutofixPentestSettings, error) {
	var wrapper struct {
		Settings AutofixPentestSettings `json:"settings"`
	}
	if err := c.getAutofixSettings(ctx, autofixPentestSettingsPath, &wrapper); err != nil {
		return nil, err
	}
	return &wrapper.Settings, nil
}

// UpdateAutofixPentestSettings updates the workspace pentest and AI code analysis
// autofix settings.
func (c *AikidoClient) UpdateAutofixPentestSettings(ctx context.Context, req AutofixPentestSettingsRequest) error {
	return c.putAutofixSettings(ctx, autofixPentestSettingsPath, req)
}

// getAutofixSettings performs a GET against an autofix settings endpoint and decodes the
// response into out.
func (c *AikidoClient) getAutofixSettings(ctx context.Context, path string, out interface{}) error {
	resp, err := c.DoRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return fmt.Errorf("getting autofix settings: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return autofixStatusError(resp.StatusCode, body, "getting autofix settings")
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decoding autofix settings response: %w", err)
	}

	return nil
}

// putAutofixSettings performs a PUT against an autofix settings endpoint. The
// {"success": 1} response body carries no extra information and is ignored.
func (c *AikidoClient) putAutofixSettings(ctx context.Context, path string, body interface{}) error {
	resp, err := c.DoRequest(ctx, http.MethodPut, path, body)
	if err != nil {
		return fmt.Errorf("updating autofix settings: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return autofixStatusError(resp.StatusCode, respBody, "updating autofix settings")
	}

	return nil
}

// autofixStatusError converts a non-200 autofix response into an error, tagging the
// workspace-level disablement case with ErrAutofixDisabledForWorkspace so callers can
// detect it with errors.Is rather than matching on message text.
func autofixStatusError(status int, body []byte, action string) error {
	msg := errorBody(body)
	if status == http.StatusBadRequest && strings.Contains(strings.ToLower(msg), "autofix has been disabled") {
		return fmt.Errorf("%w: %s", ErrAutofixDisabledForWorkspace, msg)
	}
	return fmt.Errorf("unexpected status %d %s: %s", status, action, msg)
}
