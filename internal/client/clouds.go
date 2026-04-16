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

// Cloud represents a cloud environment in the Aikido API.
type Cloud struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	Environment string `json:"environment"`
	ExternalID  string `json:"external_id"`
}

// CreateCloudResponse is the response from creating a cloud environment.
type CreateCloudResponse struct {
	ID int `json:"id"`
}

// CreateAWSCloudRequest is the request body for connecting an AWS cloud.
type CreateAWSCloudRequest struct {
	Name        string `json:"name"`
	Environment string `json:"environment"`
	RoleARN     string `json:"role_arn"`
}

// CreateAzureCloudRequest is the request body for connecting an Azure cloud.
type CreateAzureCloudRequest struct {
	Name           string `json:"name"`
	Environment    string `json:"environment"`
	ApplicationID  string `json:"application_id"`
	DirectoryID    string `json:"directory_id"`
	SubscriptionID string `json:"subscription_id"`
	KeyValue       string `json:"key_value"`
}

// CreateGCPCloudRequest is the request body for connecting a GCP cloud.
type CreateGCPCloudRequest struct {
	Name        string `json:"name"`
	Environment string `json:"environment"`
	ProjectID   string `json:"project_id"`
	AccessKey   string `json:"access_key"`
}

// CreateKubernetesCloudRequest is the request body for connecting a Kubernetes cloud.
type CreateKubernetesCloudRequest struct {
	Name                string   `json:"name"`
	Environment         string   `json:"environment"`
	ExcludedNamespaces  []string `json:"excluded_namespaces,omitempty"`
	IncludedNamespaces  []string `json:"included_namespaces,omitempty"`
	EnableImageScanning bool     `json:"enable_image_scanning,omitempty"`
}

// KubernetesCloudResponse is the response from creating a Kubernetes cloud.
type KubernetesCloudResponse struct {
	ID         int    `json:"id"`
	Endpoint   string `json:"endpoint"`
	AgentToken string `json:"agent_token"`
	CreatedAt  int64  `json:"created_at"`
}

// GetCloud retrieves a single cloud by ID by paginating through the list endpoint.
func (c *AikidoClient) GetCloud(ctx context.Context, cloudID int) (*Cloud, error) {
	page := 0
	perPage := 20
	for {
		params := url.Values{}
		params.Set("page", strconv.Itoa(page))
		params.Set("per_page", strconv.Itoa(perPage))

		clouds, err := c.getCloudsPage(ctx, params)
		if err != nil {
			return nil, err
		}

		if len(clouds) == 0 {
			return nil, fmt.Errorf("cloud with ID %d not found", cloudID)
		}

		for i := range clouds {
			if clouds[i].ID == cloudID {
				return &clouds[i], nil
			}
		}

		if len(clouds) < perPage {
			return nil, fmt.Errorf("cloud with ID %d not found", cloudID)
		}

		page++
	}
}

// ListClouds returns all cloud environments by paginating through every page.
func (c *AikidoClient) ListClouds(ctx context.Context) ([]Cloud, error) {
	var allClouds []Cloud
	page := 0
	perPage := 20

	for {
		params := url.Values{}
		params.Set("page", strconv.Itoa(page))
		params.Set("per_page", strconv.Itoa(perPage))

		clouds, err := c.getCloudsPage(ctx, params)
		if err != nil {
			return nil, err
		}

		if len(clouds) == 0 {
			break
		}

		allClouds = append(allClouds, clouds...)

		if len(clouds) < perPage {
			break
		}

		page++
	}

	return allClouds, nil
}

// createCloud posts to a cloud provider endpoint and returns the created ID.
func (c *AikidoClient) createCloud(ctx context.Context, path string, body interface{}) (int, error) {
	resp, err := c.DoRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return 0, fmt.Errorf("creating cloud: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("unexpected status %d creating cloud: %s", resp.StatusCode, string(respBody))
	}

	var createResp CreateCloudResponse
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		return 0, fmt.Errorf("decoding create cloud response: %w", err)
	}

	return createResp.ID, nil
}

// CreateAWSCloud connects an AWS cloud environment.
func (c *AikidoClient) CreateAWSCloud(ctx context.Context, req CreateAWSCloudRequest) (int, error) {
	return c.createCloud(ctx, "/clouds/aws", req)
}

// CreateAzureCloud connects an Azure cloud environment.
func (c *AikidoClient) CreateAzureCloud(ctx context.Context, req CreateAzureCloudRequest) (int, error) {
	return c.createCloud(ctx, "/clouds/azure", req)
}

// CreateGCPCloud connects a GCP cloud environment.
func (c *AikidoClient) CreateGCPCloud(ctx context.Context, req CreateGCPCloudRequest) (int, error) {
	return c.createCloud(ctx, "/clouds/gcp", req)
}

// CreateKubernetesCloud connects a Kubernetes cloud environment.
func (c *AikidoClient) CreateKubernetesCloud(ctx context.Context, req CreateKubernetesCloudRequest) (*KubernetesCloudResponse, error) {
	resp, err := c.DoRequest(ctx, http.MethodPost, "/clouds/kubernetes", req)
	if err != nil {
		return nil, fmt.Errorf("creating kubernetes cloud: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d creating kubernetes cloud: %s", resp.StatusCode, errorBody(body))
	}

	var k8sResp KubernetesCloudResponse
	if err := json.NewDecoder(resp.Body).Decode(&k8sResp); err != nil {
		return nil, fmt.Errorf("decoding kubernetes cloud response: %w", err)
	}

	return &k8sResp, nil
}

// DeleteCloud deletes a cloud environment by ID.
func (c *AikidoClient) DeleteCloud(ctx context.Context, cloudID int) error {
	resp, err := c.DoRequest(ctx, http.MethodDelete, fmt.Sprintf("/clouds/%d", cloudID), nil)
	if err != nil {
		return fmt.Errorf("deleting cloud: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil // Already deleted
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d deleting cloud: %s", resp.StatusCode, errorBody(body))
	}

	return nil
}

// getCloudsPage fetches a single page of cloud environments.
func (c *AikidoClient) getCloudsPage(ctx context.Context, params url.Values) ([]Cloud, error) {
	resp, err := c.DoRequest(ctx, http.MethodGet, "/clouds?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("listing clouds: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d listing clouds: %s", resp.StatusCode, errorBody(body))
	}

	var clouds []Cloud
	if err := json.NewDecoder(resp.Body).Decode(&clouds); err != nil {
		return nil, fmt.Errorf("decoding clouds response: %w", err)
	}

	return clouds, nil
}
