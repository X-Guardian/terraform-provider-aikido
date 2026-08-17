package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

// containersPageSize is the per_page value used when listing containers.
// Matches the API maximum documented at GET /containers (per_page max 100).
const containersPageSize = 100

// ContainerLabel represents a label attached to a container repository.
type ContainerLabel struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	IsImported bool   `json:"is_imported"`
}

// Container represents a container repository in the Aikido API (list response).
//
// Sensitivity, Connectivity, IsRunning and Labels are only populated when the corresponding
// include option is set on the request, so they are pointers to distinguish "not requested"
// from a zero value the API genuinely returned.
type Container struct {
	ID               int              `json:"id"`
	Name             string           `json:"name"`
	Provider         string           `json:"provider"`
	CloudID          *int             `json:"cloud_id"`
	RegistryID       *int             `json:"registry_id"`
	RegistryName     *string          `json:"registry_name"`
	Tag              string           `json:"tag"`
	Distro           string           `json:"distro"`
	DistroVersion    string           `json:"distro_version"`
	LastScannedAt    int64            `json:"last_scanned_at"`
	LastScannedTag   string           `json:"last_scanned_tag"`
	LinkedCodeRepoID *int             `json:"linked_code_repo_id"`
	LastPushedAt     int64            `json:"last_pushed_at"`
	CreatedAt        int64            `json:"created_at"`
	IsActive         bool             `json:"is_active"`
	IsEmpty          bool             `json:"is_empty"`
	ExposedVia       string           `json:"exposed_via"`
	Sensitivity      *string          `json:"sensitivity"`
	Connectivity     *string          `json:"connectivity"`
	IsRunning        *bool            `json:"is_running"`
	Labels           []ContainerLabel `json:"labels"`
}

// ContainerDetail represents the detailed response from GET /containers/{id}.
//
// The detail endpoint returns a narrower set of fields than the list endpoint: notably it
// carries IsActive but not sensitivity or connectivity, which are available only from
// GET /containers with the matching include options.
type ContainerDetail struct {
	ID               int              `json:"id"`
	Name             string           `json:"name"`
	Provider         string           `json:"provider"`
	CloudID          *int             `json:"cloud_id"`
	RegistryID       *int             `json:"registry_id"`
	RegistryName     *string          `json:"registry_name"`
	Tag              string           `json:"tag"`
	Distro           string           `json:"distro"`
	DistroVersion    string           `json:"distro_version"`
	LastScannedAt    int64            `json:"last_scanned_at"`
	LastScannedTag   string           `json:"last_scanned_tag"`
	LinkedCodeRepoID *int             `json:"linked_code_repo_id"`
	IsActive         bool             `json:"is_active"`
	Labels           []ContainerLabel `json:"labels"`
}

// ListContainersOptions contains optional filters for listing containers.
//
// The Include options control whether fields the API omits by default are returned. Each one
// widens the response rather than filtering it.
type ListContainersOptions struct {
	FilterName         string
	FilterTag          string
	FilterTeamID       *int
	FilterStatus       string // "active" (default), "inactive", "all"
	FilterReachability string // "unknown", "direct", "lb", "limited_ips", "none"

	IncludeIsRunning    bool
	IncludeSensitivity  bool
	IncludeConnectivity bool
	IncludeLabels       bool
}

// apply writes the options onto a query string. A nil receiver writes nothing.
func (o *ListContainersOptions) apply(params url.Values) {
	if o == nil {
		return
	}

	if o.FilterName != "" {
		params.Set("filter_name", o.FilterName)
	}
	if o.FilterTag != "" {
		params.Set("filter_tag", o.FilterTag)
	}
	if o.FilterTeamID != nil {
		params.Set("filter_team_id", strconv.Itoa(*o.FilterTeamID))
	}
	if o.FilterStatus != "" {
		params.Set("filter_status", o.FilterStatus)
	}
	if o.FilterReachability != "" {
		params.Set("filter_reachability", o.FilterReachability)
	}
	if o.IncludeIsRunning {
		params.Set("include_is_running", "true")
	}
	if o.IncludeSensitivity {
		params.Set("include_sensitivity", "true")
	}
	if o.IncludeConnectivity {
		params.Set("include_connectivity", "true")
	}
	if o.IncludeLabels {
		params.Set("include_labels", "true")
	}
}

// GetContainer retrieves a single container by ID.
func (c *AikidoClient) GetContainer(ctx context.Context, containerID int) (*ContainerDetail, error) {
	resp, err := c.DoRequest(ctx, http.MethodGet, fmt.Sprintf("/containers/%d", containerID), nil)
	if err != nil {
		return nil, fmt.Errorf("getting container: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("container with ID %d: %w", containerID, ErrContainerNotFound)
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

// ErrContainerNotFound is returned when a container cannot be found.
var ErrContainerNotFound = errors.New("container not found")

// FindContainer returns a single container from the list endpoint, matched on its ID.
//
// GET /containers/{id} omits the sensitivity and connectivity fields, so a full read has to go
// through the list endpoint, which returns them when asked. Matching on ID rather than a name
// filter keeps the lookup exact, and filter_status is forced to "all" so a container that was
// deactivated outside Terraform is still found and reported as inactive rather than missing.
//
// Results are served from a short-lived cache of the full container list, shared across
// concurrent callers, so refreshing N container resources costs one paginated scan rather than N.
// Every container write invalidates that cache, so a read-back after a write always refetches.
//
// nameHint narrows the fallback search to containers matching that name, which usually reduces
// the scan to a single page. It only applies when the cache cannot answer the lookup, and it is
// only an optimisation: the ID match still decides the result, so a substring or otherwise
// inexact filter cannot select the wrong container. Pass an empty string when the name is
// unknown, as it is on the first read after an import. A hinted search that finds nothing falls
// back to an unfiltered scan, so a stale or renamed hint cannot make a container that does exist
// look missing.
//
// The returned error wraps ErrContainerNotFound when no container has the given ID.
func (c *AikidoClient) FindContainer(ctx context.Context, containerID int, nameHint string) (*Container, error) {
	// A cached listing already covers every container, so a miss against a fresh one is
	// authoritative and needs no further requests.
	if entry, ok := c.containersCache.get(); ok {
		if container, found := entry.containersByID[containerID]; found {
			return container, nil
		}
		return nil, fmt.Errorf("container with ID %d: %w", containerID, ErrContainerNotFound)
	}

	// A name hint can answer the lookup without fetching every container, so it is tried before
	// populating the cache. The result is deliberately not cached: it is a partial listing.
	if nameHint != "" {
		container, err := c.findContainerByID(ctx, containerID, nameHint)
		if err == nil {
			return container, nil
		}
		// Anything other than a miss is a real failure; only a miss warrants a second attempt.
		if !errors.Is(err, ErrContainerNotFound) {
			return nil, err
		}
	}

	entry, err := c.containersCache.getOrFetch(ctx, func() ([]Container, error) {
		return c.listAllContainersForCache(ctx)
	})
	if err != nil {
		return nil, err
	}

	if container, found := entry.containersByID[containerID]; found {
		return container, nil
	}

	return nil, fmt.Errorf("container with ID %d: %w", containerID, ErrContainerNotFound)
}

// listAllContainersForCache fetches every container with the fields a configuration read needs.
func (c *AikidoClient) listAllContainersForCache(ctx context.Context) ([]Container, error) {
	return c.ListContainers(ctx, &ListContainersOptions{
		FilterStatus:        "all",
		IncludeIsRunning:    true,
		IncludeSensitivity:  true,
		IncludeConnectivity: true,
		IncludeLabels:       true,
	})
}

// findContainerByID scans the list endpoint for a container with the given ID, optionally
// narrowing the request with a name filter.
func (c *AikidoClient) findContainerByID(ctx context.Context, containerID int, filterName string) (*Container, error) {
	opts := ListContainersOptions{
		FilterName:          filterName,
		FilterStatus:        "all",
		IncludeIsRunning:    true,
		IncludeSensitivity:  true,
		IncludeConnectivity: true,
		IncludeLabels:       true,
	}

	page := 0
	for {
		params := url.Values{}
		params.Set("page", strconv.Itoa(page))
		params.Set("per_page", strconv.Itoa(containersPageSize))
		opts.apply(params)

		containers, err := c.getContainersPage(ctx, params)
		if err != nil {
			return nil, err
		}

		for i := range containers {
			if containers[i].ID == containerID {
				return &containers[i], nil
			}
		}

		if len(containers) < containersPageSize {
			return nil, fmt.Errorf("container with ID %d: %w", containerID, ErrContainerNotFound)
		}

		page++
	}
}

// ListContainers returns all containers by paginating through every page.
func (c *AikidoClient) ListContainers(ctx context.Context, opts *ListContainersOptions) ([]Container, error) {
	var allContainers []Container
	page := 0

	for {
		params := url.Values{}
		params.Set("page", strconv.Itoa(page))
		params.Set("per_page", strconv.Itoa(containersPageSize))
		opts.apply(params)

		containers, err := c.getContainersPage(ctx, params)
		if err != nil {
			return nil, err
		}

		if len(containers) == 0 {
			break
		}

		allContainers = append(allContainers, containers...)

		if len(containers) < containersPageSize {
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

	c.containersCache.invalidate(ctx)

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

	c.containersCache.invalidate(ctx)

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

	c.containersCache.invalidate(ctx)

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

	c.containersCache.invalidate(ctx)

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

	c.containersCache.invalidate(ctx)

	return nil
}

// LinkCodeRepoToContainer links a code repository to a container.
func (c *AikidoClient) LinkCodeRepoToContainer(ctx context.Context, containerID, codeRepoID int) error {
	body := map[string]int{
		"container_repo_id": containerID,
		"code_repo_id":      codeRepoID,
	}

	resp, err := c.DoRequest(ctx, http.MethodPost, "/containers/linkCodeRepo", body)
	if err != nil {
		return fmt.Errorf("linking code repo to container: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d linking code repo to container: %s", resp.StatusCode, errorBody(respBody))
	}

	c.containersCache.invalidate(ctx)

	return nil
}

// UnlinkCodeRepoFromContainer removes the code repository link from a container.
func (c *AikidoClient) UnlinkCodeRepoFromContainer(ctx context.Context, containerID int) error {
	resp, err := c.DoRequest(ctx, http.MethodPost, "/containers/unlinkCodeRepo", map[string]int{"container_repo_id": containerID})
	if err != nil {
		return fmt.Errorf("unlinking code repo from container: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d unlinking code repo from container: %s", resp.StatusCode, errorBody(respBody))
	}

	c.containersCache.invalidate(ctx)

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
