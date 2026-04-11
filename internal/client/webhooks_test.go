package client

import (
	"context"
	"net/http"
	"testing"
)

func TestCreateWebhook(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/public/v1/webhooks" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var req CreateWebhookRequest
		mustDecode(t, r, &req)
		if req.TargetURL != "https://example.com/hook" {
			t.Errorf("expected target_url 'https://example.com/hook', got %q", req.TargetURL)
		}
		if req.EventType != "issue.open.created" {
			t.Errorf("expected event_type 'issue.open.created', got %q", req.EventType)
		}
		mustEncode(t, w, CreateWebhookResponse{WebhookID: 42})
	})
	defer server.Close()

	id, err := c.CreateWebhook(context.Background(), CreateWebhookRequest{
		TargetURL: "https://example.com/hook",
		EventType: "issue.open.created",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 42 {
		t.Errorf("expected ID 42, got %d", id)
	}
}

func TestGetWebhook_Found(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		mustEncode(t, w, []Webhook{
			{ID: "1", TargetURL: "https://a.com/hook", EventType: "issue.closed", HealthStatus: "success", LatestHTTPStatusCode: 200},
			{ID: "2", TargetURL: "https://b.com/hook", EventType: "zen.attack", HealthStatus: "unknown", LatestHTTPStatusCode: 0},
		})
	})
	defer server.Close()

	webhook, err := c.GetWebhook(context.Background(), "2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if webhook.TargetURL != "https://b.com/hook" {
		t.Errorf("expected target_url 'https://b.com/hook', got %q", webhook.TargetURL)
	}
}

func TestGetWebhook_NotFound(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		mustEncode(t, w, []Webhook{})
	})
	defer server.Close()

	_, err := c.GetWebhook(context.Background(), "999")
	if err == nil {
		t.Fatal("expected error for not found")
	}
}

func TestListWebhooks(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		mustEncode(t, w, []Webhook{
			{ID: "1", TargetURL: "https://a.com/hook", EventType: "issue.closed"},
			{ID: "2", TargetURL: "https://b.com/hook", EventType: "zen.attack"},
		})
	})
	defer server.Close()

	webhooks, err := c.ListWebhooks(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(webhooks) != 2 {
		t.Errorf("expected 2 webhooks, got %d", len(webhooks))
	}
}

func TestDeleteWebhook(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/api/public/v1/webhooks/42" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		mustEncode(t, w, map[string]int{"success": 1})
	})
	defer server.Close()

	err := c.DeleteWebhook(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
