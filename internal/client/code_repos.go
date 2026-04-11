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
)

// CodeRepo represents a code repository in the Aikido API.
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

// ListCodeReposOptions contains optional filters for listing code repositories.
type ListCodeReposOptions struct {
	IncludeInactive bool
	FilterName      string
	FilterBranch    string
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
		return nil, fmt.Errorf("unexpected status %d listing code repos: %s", resp.StatusCode, string(body))
	}

	var repos []CodeRepo
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return nil, fmt.Errorf("decoding code repos response: %w", err)
	}

	return repos, nil
}
