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

// Container represents a container repository in the Aikido API (list response).
type Container struct {
	ID               int     `json:"id"`
	Name             string  `json:"name"`
	Provider         string  `json:"provider"`
	CloudID          *int    `json:"cloud_id"`
	RegistryID       *int    `json:"registry_id"`
	RegistryName     *string `json:"registry_name"`
	Tag              string  `json:"tag"`
	Distro           string  `json:"distro"`
	DistroVersion    string  `json:"distro_version"`
	LastScannedAt    int64   `json:"last_scanned_at"`
	LastScannedTag   string  `json:"last_scanned_tag"`
	LinkedCodeRepoID *int    `json:"linked_code_repo_id"`
	LastPushedAt     int64   `json:"last_pushed_at"`
	CreatedAt        int64   `json:"created_at"`
}

// ContainerDetail represents the detailed response from GET /containers/{id}.
// Currently the same fields as the list response.
type ContainerDetail = Container

// ListContainersOptions contains optional filters for listing containers.
type ListContainersOptions struct {
	FilterName   string
	FilterTag    string
	FilterTeamID *int
	FilterStatus string // "active" (default), "inactive", "all"
}

// GetContainer retrieves a single container by ID.
func (c *AikidoClient) GetContainer(ctx context.Context, containerID int) (*ContainerDetail, error) {
	resp, err := c.DoRequest(ctx, http.MethodGet, fmt.Sprintf("/containers/%d", containerID), nil)
	if err != nil {
		return nil, fmt.Errorf("getting container: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("container with ID %d not found", containerID)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d getting container: %s", resp.StatusCode, errorBody(body))
	}

	var container ContainerDetail
	if err := json.NewDecoder(resp.Body).Decode(&container); err != nil {
		return nil, fmt.Errorf("decoding container response: %w", err)
	}

	return &container, nil
}

// ListContainers returns all containers by paginating through every page.
func (c *AikidoClient) ListContainers(ctx context.Context, opts *ListContainersOptions) ([]Container, error) {
	var allContainers []Container
	page := 0
	pageSize := 200

	for {
		params := url.Values{}
		params.Set("page", strconv.Itoa(page))
		params.Set("pageSize", strconv.Itoa(pageSize))

		if opts != nil {
			if opts.FilterName != "" {
				params.Set("filter_name", opts.FilterName)
			}
			if opts.FilterTag != "" {
				params.Set("filter_tag", opts.FilterTag)
			}
			if opts.FilterTeamID != nil {
				params.Set("filter_team_id", strconv.Itoa(*opts.FilterTeamID))
			}
			if opts.FilterStatus != "" {
				params.Set("filter_status", opts.FilterStatus)
			}
		}

		containers, err := c.getContainersPage(ctx, params)
		if err != nil {
			return nil, err
		}

		if len(containers) == 0 {
			break
		}

		allContainers = append(allContainers, containers...)

		if len(containers) < pageSize {
			break
		}

		page++
	}

	return allContainers, nil
}

// ActivateContainer enables scanning for a container.
func (c *AikidoClient) ActivateContainer(ctx context.Context, containerID int) error {
	resp, err := c.DoRequest(ctx, http.MethodPost, "/containers/activate", map[string]int{"container_repo_id": containerID})
	if err != nil {
		return fmt.Errorf("activating container: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d activating container: %s", resp.StatusCode, errorBody(body))
	}

	return nil
}

// DeactivateContainer disables scanning for a container.
func (c *AikidoClient) DeactivateContainer(ctx context.Context, containerID int) error {
	resp, err := c.DoRequest(ctx, http.MethodPost, "/containers/deactivate", map[string]int{"container_repo_id": containerID})
	if err != nil {
		return fmt.Errorf("deactivating container: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d deactivating container: %s", resp.StatusCode, errorBody(body))
	}

	return nil
}

// UpdateContainerSensitivity updates the sensitivity level of a container.
func (c *AikidoClient) UpdateContainerSensitivity(ctx context.Context, containerID int, sensitivity string) error {
	resp, err := c.DoRequest(ctx, http.MethodPut, fmt.Sprintf("/containers/%d/sensitivity", containerID), map[string]string{"sensitivity": sensitivity})
	if err != nil {
		return fmt.Errorf("updating container sensitivity: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d updating container sensitivity: %s", resp.StatusCode, errorBody(body))
	}

	return nil
}

// UpdateContainerConnectivity updates the internet exposure status of a container.
func (c *AikidoClient) UpdateContainerConnectivity(ctx context.Context, containerID int, internetExposed string) error {
	resp, err := c.DoRequest(ctx, http.MethodPut, fmt.Sprintf("/containers/%d/internetConnection", containerID), map[string]string{"internet_exposed": internetExposed})
	if err != nil {
		return fmt.Errorf("updating container connectivity: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d updating container connectivity: %s", resp.StatusCode, errorBody(body))
	}

	return nil
}

// UpdateContainerTagFilter updates the tag filter for a container. Pass empty string to reset.
func (c *AikidoClient) UpdateContainerTagFilter(ctx context.Context, containerID int, tagFilter string) error {
	body := map[string]interface{}{
		"container_repo_id": containerID,
	}
	if tagFilter == "" {
		body["tag_filter"] = nil
	} else {
		body["tag_filter"] = tagFilter
	}

	resp, err := c.DoRequest(ctx, http.MethodPost, "/containers/updateTagFilter", body)
	if err != nil {
		return fmt.Errorf("updating container tag filter: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d updating tag filter: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// getContainersPage fetches a single page of containers.
func (c *AikidoClient) getContainersPage(ctx context.Context, params url.Values) ([]Container, error) {
	resp, err := c.DoRequest(ctx, http.MethodGet, "/containers?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("listing containers: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d listing containers: %s", resp.StatusCode, errorBody(body))
	}

	var containers []Container
	if err := json.NewDecoder(resp.Body).Decode(&containers); err != nil {
		return nil, fmt.Errorf("decoding containers response: %w", err)
	}

	return containers, nil
}
