package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ZenAppBlocking represents the blocking config request.
type ZenAppBlocking struct {
	Block                   bool `json:"block"`
	DisableMinimumWaitCheck bool `json:"disable_minimum_wait_check,omitempty"`
}

// ZenAppCountries represents the country blocking configuration.
type ZenAppCountries struct {
	Mode string              `json:"mode"`
	List []ZenAppCountryItem `json:"list"`
}

// ZenAppCountryItem represents a country in the blocking list.
type ZenAppCountryItem struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// ZenAppCountriesRequest is the request body for updating country blocking.
type ZenAppCountriesRequest struct {
	Mode string   `json:"mode"`
	List []string `json:"list"`
}

// ZenAppBotListItem represents a bot list subscription.
type ZenAppBotListItem struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Mode string `json:"mode"`
}

// ZenAppBotListUpdateItem represents a bot list update entry.
type ZenAppBotListUpdateItem struct {
	Code string `json:"code"`
	Mode string `json:"mode"`
}

// ZenAppIPLists represents the threat list configuration.
type ZenAppIPLists struct {
	KnownThreatActors []ZenAppIPListItem `json:"known_threat_actors"`
	Tor               ZenAppTorConfig    `json:"tor"`
}

// ZenAppIPListItem represents a known threat actor IP list.
type ZenAppIPListItem struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Mode string `json:"mode"`
}

// ZenAppTorConfig represents the Tor traffic configuration.
type ZenAppTorConfig struct {
	Mode string `json:"mode"`
}

// ZenAppIPListUpdateItem represents a threat actor list update entry.
type ZenAppIPListUpdateItem struct {
	Code string `json:"code"`
	Mode string `json:"mode"`
}

// ZenAppIPListsRequest is the request body for updating IP lists.
type ZenAppIPListsRequest struct {
	KnownThreatActors []ZenAppIPListUpdateItem `json:"known_threat_actors,omitempty"`
	Tor               *ZenAppTorConfig         `json:"tor,omitempty"`
}

// UpdateZenAppBlocking enables or disables blocking mode.
func (c *AikidoClient) UpdateZenAppBlocking(ctx context.Context, appID int, req ZenAppBlocking) error {
	resp, err := c.DoRequest(ctx, http.MethodPut, fmt.Sprintf("/firewall/apps/%d/blocking", appID), req)
	if err != nil {
		return fmt.Errorf("updating zen app blocking: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d updating blocking: %s", resp.StatusCode, errorBody(body))
	}

	return nil
}

// GetZenAppCountries retrieves the country blocking configuration.
func (c *AikidoClient) GetZenAppCountries(ctx context.Context, appID int) (*ZenAppCountries, error) {
	resp, err := c.DoRequest(ctx, http.MethodGet, fmt.Sprintf("/firewall/apps/%d/countries", appID), nil)
	if err != nil {
		return nil, fmt.Errorf("getting zen app countries: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d getting countries: %s", resp.StatusCode, errorBody(body))
	}

	var countries ZenAppCountries
	if err := json.NewDecoder(resp.Body).Decode(&countries); err != nil {
		return nil, fmt.Errorf("decoding countries response: %w", err)
	}

	return &countries, nil
}

// UpdateZenAppCountries updates the country blocking configuration.
func (c *AikidoClient) UpdateZenAppCountries(ctx context.Context, appID int, req ZenAppCountriesRequest) error {
	resp, err := c.DoRequest(ctx, http.MethodPut, fmt.Sprintf("/firewall/apps/%d/countries", appID), req)
	if err != nil {
		return fmt.Errorf("updating zen app countries: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d updating countries: %s", resp.StatusCode, errorBody(body))
	}

	return nil
}

// UpdateZenAppIPBlocklist updates the custom IP blocklist.
func (c *AikidoClient) UpdateZenAppIPBlocklist(ctx context.Context, appID int, ipAddresses []string) error {
	resp, err := c.DoRequest(ctx, http.MethodPut, fmt.Sprintf("/firewall/apps/%d/ip-blocklist", appID), map[string][]string{"ip_addresses": ipAddresses})
	if err != nil {
		return fmt.Errorf("updating zen app IP blocklist: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d updating IP blocklist: %s", resp.StatusCode, errorBody(body))
	}

	return nil
}

// GetZenAppBotLists retrieves the bot list configuration.
func (c *AikidoClient) GetZenAppBotLists(ctx context.Context, appID int) ([]ZenAppBotListItem, error) {
	resp, err := c.DoRequest(ctx, http.MethodGet, fmt.Sprintf("/firewall/apps/%d/bot-lists", appID), nil)
	if err != nil {
		return nil, fmt.Errorf("getting zen app bot lists: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d getting bot lists: %s", resp.StatusCode, errorBody(body))
	}

	var items []ZenAppBotListItem
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, fmt.Errorf("decoding bot lists response: %w", err)
	}

	return items, nil
}

// UpdateZenAppBotLists updates the bot list configuration.
func (c *AikidoClient) UpdateZenAppBotLists(ctx context.Context, appID int, items []ZenAppBotListUpdateItem) error {
	resp, err := c.DoRequest(ctx, http.MethodPut, fmt.Sprintf("/firewall/apps/%d/bot-lists", appID), items)
	if err != nil {
		return fmt.Errorf("updating zen app bot lists: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d updating bot lists: %s", resp.StatusCode, errorBody(body))
	}

	return nil
}

// GetZenAppIPLists retrieves the threat list configuration.
func (c *AikidoClient) GetZenAppIPLists(ctx context.Context, appID int) (*ZenAppIPLists, error) {
	resp, err := c.DoRequest(ctx, http.MethodGet, fmt.Sprintf("/firewall/apps/%d/ip-lists", appID), nil)
	if err != nil {
		return nil, fmt.Errorf("getting zen app IP lists: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d getting IP lists: %s", resp.StatusCode, errorBody(body))
	}

	var lists ZenAppIPLists
	if err := json.NewDecoder(resp.Body).Decode(&lists); err != nil {
		return nil, fmt.Errorf("decoding IP lists response: %w", err)
	}

	return &lists, nil
}

// UpdateZenAppIPLists updates the threat list configuration.
func (c *AikidoClient) UpdateZenAppIPLists(ctx context.Context, appID int, req ZenAppIPListsRequest) error {
	resp, err := c.DoRequest(ctx, http.MethodPut, fmt.Sprintf("/firewall/apps/%d/ip-lists", appID), req)
	if err != nil {
		return fmt.Errorf("updating zen app IP lists: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d updating IP lists: %s", resp.StatusCode, errorBody(body))
	}

	return nil
}
