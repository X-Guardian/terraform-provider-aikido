package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// CodeRepo represents a code repository in the Aikido API (list response).
type CodeRepo struct {
	ID                    int    `json:"id"`
	Name                  string `json:"name"`
	Provider              string `json:"provider"`
	ExternalRepoID        string `json:"external_repo_id"`
	ExternalRepoNumericID int64  `json:"external_repo_numeric_id"`
	Active                bool   `json:"active"`
	URL                   string `json:"url"`
	Branch                string `json:"branch"`
	LastScannedAt         int64  `json:"last_scanned_at"`
	Connectivity          string `json:"connectivity"`
	Sensitivity           string `json:"sensitivity"`
}

// CodeRepoDetail represents the detailed response from GET /repositories/code/{id}.
type CodeRepoDetail struct {
	ID                    int            `json:"id"`
	Name                  string         `json:"name"`
	Provider              string         `json:"provider"`
	ExternalRepoID        string         `json:"external_repo_id"`
	ExternalRepoNumericID int64          `json:"external_repo_numeric_id"`
	Active                bool           `json:"active"`
	URL                   string         `json:"url"`
	Branch                string         `json:"branch"`
	LastScannedAt         int64          `json:"last_scanned_at"`
	Connectivity          string         `json:"connectivity"`
	Sensitivity           string         `json:"sensitivity"`
	ExcludedPaths         []ExcludedPath `json:"excluded_paths"`
}

// ExcludedPath represents a path excluded from scanning.
type ExcludedPath struct {
	ID   int    `json:"id"`
	Path string `json:"path"`
}

// ListCodeReposOptions contains optional filters for listing code repositories.
type ListCodeReposOptions struct {
	IncludeInactive bool
	FilterName      string
	FilterBranch    string
}

// GetCodeRepo retrieves a single code repository by ID.
func (c *AikidoClient) GetCodeRepo(ctx context.Context, repoID int) (*CodeRepoDetail, error) {
	resp, err := c.DoRequest(ctx, http.MethodGet, fmt.Sprintf("/repositories/code/%d", repoID), nil)
	if err != nil {
		return nil, fmt.Errorf("getting code repo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("code repo with ID %d not found", repoID)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d getting code repo: %s", resp.StatusCode, errorBody(body))
	}

	var repo CodeRepoDetail
	if err := json.NewDecoder(resp.Body).Decode(&repo); err != nil {
		return nil, fmt.Errorf("decoding code repo response: %w", err)
	}

	return &repo, nil
}

// ActivateCodeRepo enables scanning for a code repository.
func (c *AikidoClient) ActivateCodeRepo(ctx context.Context, repoID int) error {
	resp, err := c.DoRequest(ctx, http.MethodPost, "/repositories/code/activate", map[string]int{"code_repo_id": repoID})
	if err != nil {
		return fmt.Errorf("activating code repo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d activating code repo: %s", resp.StatusCode, errorBody(body))
	}

	return nil
}

// DeactivateCodeRepo disables scanning for a code repository.
func (c *AikidoClient) DeactivateCodeRepo(ctx context.Context, repoID int) error {
	resp, err := c.DoRequest(ctx, http.MethodPost, "/repositories/code/deactivate", map[string]int{"code_repo_id": repoID})
	if err != nil {
		return fmt.Errorf("deactivating code repo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d deactivating code repo: %s", resp.StatusCode, errorBody(body))
	}

	return nil
}

// UpdateCodeRepoSensitivity updates the sensitivity level of a code repository.
func (c *AikidoClient) UpdateCodeRepoSensitivity(ctx context.Context, repoID int, sensitivity string) error {
	resp, err := c.DoRequest(ctx, http.MethodPut, fmt.Sprintf("/repositories/code/%d/sensitivity", repoID), map[string]string{"sensitivity": sensitivity})
	if err != nil {
		return fmt.Errorf("updating code repo sensitivity: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d updating sensitivity: %s", resp.StatusCode, errorBody(body))
	}

	return nil
}

// UpdateCodeRepoConnectivity updates the connectivity status of a code repository.
func (c *AikidoClient) UpdateCodeRepoConnectivity(ctx context.Context, repoID int, connectivity string) error {
	resp, err := c.DoRequest(ctx, http.MethodPut, fmt.Sprintf("/repositories/code/%d/connectivity", repoID), map[string]string{"connectivity": connectivity})
	if err != nil {
		return fmt.Errorf("updating code repo connectivity: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d updating connectivity: %s", resp.StatusCode, errorBody(body))
	}

	return nil
}

// UpdateCodeRepoDevDepScanning updates dev dependency scanning for a code repository.
func (c *AikidoClient) UpdateCodeRepoDevDepScanning(ctx context.Context, repoID int, enabled bool) error {
	resp, err := c.DoRequest(ctx, http.MethodPut, fmt.Sprintf("/repositories/code/%d/devdep-scan", repoID), map[string]bool{"dev_dep_scanning_enabled": enabled})
	if err != nil {
		return fmt.Errorf("updating dev dep scanning: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d updating dev dep scanning: %s", resp.StatusCode, errorBody(body))
	}

	return nil
}

// AddCodeRepoExcludePath adds an excluded path to a code repository.
func (c *AikidoClient) AddCodeRepoExcludePath(ctx context.Context, repoID int, path string) error {
	resp, err := c.DoRequest(ctx, http.MethodPost, fmt.Sprintf("/repositories/code/%d/exclude-path", repoID), map[string]string{"path": path})
	if err != nil {
		return fmt.Errorf("adding exclude path: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d adding exclude path: %s", resp.StatusCode, errorBody(body))
	}

	return nil
}

// RemoveCodeRepoExcludePath removes an excluded path from a code repository.
func (c *AikidoClient) RemoveCodeRepoExcludePath(ctx context.Context, repoID int, path string) error {
	resp, err := c.DoRequest(ctx, http.MethodPost, fmt.Sprintf("/repositories/code/%d/exclude-path/remove", repoID), map[string]string{"path": path})
	if err != nil {
		return fmt.Errorf("removing exclude path: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d removing exclude path: %s", resp.StatusCode, errorBody(body))
	}

	return nil
}

// ListCodeRepos returns all code repositories by paginating through every page.
func (c *AikidoClient) ListCodeRepos(ctx context.Context, opts *ListCodeReposOptions) ([]CodeRepo, error) {
	var allRepos []CodeRepo
	page := 0
	perPage := 200

	for {
		params := url.Values{}
		params.Set("page", fmt.Sprintf("%d", page))
		params.Set("per_page", fmt.Sprintf("%d", perPage))

		if opts != nil {
			if opts.IncludeInactive {
				params.Set("include_inactive", "true")
			}
			if opts.FilterName != "" {
				params.Set("filter_name", opts.FilterName)
			}
			if opts.FilterBranch != "" {
				params.Set("filter_branch", opts.FilterBranch)
			}
		}

		repos, err := c.getCodeReposPage(ctx, params)
		if err != nil {
			return nil, err
		}

		if len(repos) == 0 {
			break
		}

		allRepos = append(allRepos, repos...)

		if len(repos) < perPage {
			break
		}

		page++
	}

	return allRepos, nil
}

// getCodeReposPage fetches a single page of code repositories.
func (c *AikidoClient) getCodeReposPage(ctx context.Context, params url.Values) ([]CodeRepo, error) {
	resp, err := c.DoRequest(ctx, http.MethodGet, "/repositories/code?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("listing code repos: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d listing code repos: %s", resp.StatusCode, errorBody(body))
	}

	var repos []CodeRepo
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return nil, fmt.Errorf("decoding code repos response: %w", err)
	}

	return repos, nil
}
