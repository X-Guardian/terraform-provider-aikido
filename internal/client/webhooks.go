package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Webhook represents a webhook in the Aikido API.
type Webhook struct {
	ID                   string `json:"id"`
	TargetURL            string `json:"target_url"`
	EventType            string `json:"event_type"`
	HealthStatus         string `json:"health_status"`
	LatestHTTPStatusCode int    `json:"latest_http_status_code"`
}

// CreateWebhookRequest is the request body for creating a webhook.
type CreateWebhookRequest struct {
	TargetURL string `json:"target_url"`
	EventType string `json:"event_type"`
}

// CreateWebhookResponse is the response from creating a webhook.
type CreateWebhookResponse struct {
	WebhookID int `json:"webhook_id"`
}

// CreateWebhook creates a new webhook and returns its ID.
func (c *AikidoClient) CreateWebhook(ctx context.Context, req CreateWebhookRequest) (int, error) {
	resp, err := c.DoRequest(ctx, http.MethodPost, "/webhooks", req)
	if err != nil {
		return 0, fmt.Errorf("creating webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("unexpected status %d creating webhook: %s", resp.StatusCode, errorBody(body))
	}

	var createResp CreateWebhookResponse
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		return 0, fmt.Errorf("decoding create webhook response: %w", err)
	}

	return createResp.WebhookID, nil
}

// GetWebhook retrieves a single webhook by ID from the list endpoint.
func (c *AikidoClient) GetWebhook(ctx context.Context, webhookID string) (*Webhook, error) {
	webhooks, err := c.ListWebhooks(ctx)
	if err != nil {
		return nil, err
	}

	for i := range webhooks {
		if webhooks[i].ID == webhookID {
			return &webhooks[i], nil
		}
	}

	return nil, fmt.Errorf("webhook with ID %s not found", webhookID)
}

// ListWebhooks returns all webhooks.
func (c *AikidoClient) ListWebhooks(ctx context.Context) ([]Webhook, error) {
	resp, err := c.DoRequest(ctx, http.MethodGet, "/webhooks", nil)
	if err != nil {
		return nil, fmt.Errorf("listing webhooks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d listing webhooks: %s", resp.StatusCode, errorBody(body))
	}

	var webhooks []Webhook
	if err := json.NewDecoder(resp.Body).Decode(&webhooks); err != nil {
		return nil, fmt.Errorf("decoding webhooks response: %w", err)
	}

	return webhooks, nil
}

// DeleteWebhook deletes a webhook by ID.
func (c *AikidoClient) DeleteWebhook(ctx context.Context, webhookID int) error {
	resp, err := c.DoRequest(ctx, http.MethodDelete, fmt.Sprintf("/webhooks/%d", webhookID), nil)
	if err != nil {
		return fmt.Errorf("deleting webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d deleting webhook: %s", resp.StatusCode, errorBody(body))
	}

	return nil
}
