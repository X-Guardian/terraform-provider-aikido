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

// Domain represents a domain in the Aikido API.
type Domain struct {
	ID     int    `json:"id"`
	Domain string `json:"domain"`
	Kind   string `json:"kind"`
}

// CreateDomainRequest is the request body for creating a domain.
type CreateDomainRequest struct {
	Domain         string `json:"domain"`
	Kind           string `json:"kind"`
	OpenAPISpecURL string `json:"openapi_spec_url,omitempty"`
}

// CreateDomainResponse is the response from creating a domain.
type CreateDomainResponse struct {
	ID int `json:"id"`
}

// CreateDomain creates a new domain and returns its ID.
func (c *AikidoClient) CreateDomain(ctx context.Context, req CreateDomainRequest) (int, error) {
	resp, err := c.DoRequest(ctx, http.MethodPost, "/domains", req)
	if err != nil {
		return 0, fmt.Errorf("creating domain: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("unexpected status %d creating domain: %s", resp.StatusCode, errorBody(body))
	}

	var createResp CreateDomainResponse
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		return 0, fmt.Errorf("decoding create domain response: %w", err)
	}

	return createResp.ID, nil
}

// GetDomain retrieves a single domain by ID by paginating through the list endpoint.
func (c *AikidoClient) GetDomain(ctx context.Context, domainID int) (*Domain, error) {
	page := 0
	pageSize := 20
	for {
		params := url.Values{}
		params.Set("page", strconv.Itoa(page))
		params.Set("page_size", strconv.Itoa(pageSize))

		domains, err := c.getDomainsPage(ctx, params)
		if err != nil {
			return nil, err
		}

		if len(domains) == 0 {
			return nil, fmt.Errorf("domain with ID %d not found", domainID)
		}

		for i := range domains {
			if domains[i].ID == domainID {
				return &domains[i], nil
			}
		}

		if len(domains) < pageSize {
			return nil, fmt.Errorf("domain with ID %d not found", domainID)
		}

		page++
	}
}

// ListDomains returns all domains by paginating through every page.
func (c *AikidoClient) ListDomains(ctx context.Context) ([]Domain, error) {
	var allDomains []Domain
	page := 0
	pageSize := 20

	for {
		params := url.Values{}
		params.Set("page", strconv.Itoa(page))
		params.Set("page_size", strconv.Itoa(pageSize))

		domains, err := c.getDomainsPage(ctx, params)
		if err != nil {
			return nil, err
		}

		if len(domains) == 0 {
			break
		}

		allDomains = append(allDomains, domains...)

		if len(domains) < pageSize {
			break
		}

		page++
	}

	return allDomains, nil
}

// DeleteDomain deletes a domain by ID.
func (c *AikidoClient) DeleteDomain(ctx context.Context, domainID int) error {
	resp, err := c.DoRequest(ctx, http.MethodDelete, fmt.Sprintf("/domains/%d", domainID), nil)
	if err != nil {
		return fmt.Errorf("deleting domain: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil // Already deleted
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d deleting domain: %s", resp.StatusCode, errorBody(body))
	}

	return nil
}

// getDomainsPage fetches a single page of domains.
func (c *AikidoClient) getDomainsPage(ctx context.Context, params url.Values) ([]Domain, error) {
	resp, err := c.DoRequest(ctx, http.MethodGet, "/domains?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("listing domains: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d listing domains: %s", resp.StatusCode, errorBody(body))
	}

	var domains []Domain
	if err := json.NewDecoder(resp.Body).Decode(&domains); err != nil {
		return nil, fmt.Errorf("decoding domains response: %w", err)
	}

	return domains, nil
}
