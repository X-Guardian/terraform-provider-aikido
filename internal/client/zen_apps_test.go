package client

import (
	"context"
	"net/http"
	"testing"
)

func TestCreateZenApp(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/public/v1/firewall/apps" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var req CreateZenAppRequest
		mustDecode(t, r, &req)
		if req.Name != "my-app" {
			t.Errorf("expected name 'my-app', got %q", req.Name)
		}
		mustEncode(t, w, CreateZenAppResponse{AppID: 42, Token: "zen-token-123"})
	})
	defer server.Close()

	resp, err := c.CreateZenApp(context.Background(), CreateZenAppRequest{
		Name:        "my-app",
		Environment: "production",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.AppID != 42 {
		t.Errorf("expected app_id 42, got %d", resp.AppID)
	}
	if resp.Token != "zen-token-123" {
		t.Errorf("expected token 'zen-token-123', got %q", resp.Token)
	}
}

func TestGetZenApp(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/firewall/apps/42" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		mustEncode(t, w, ZenAppDetail{
			ID:          "42",
			Name:        "my-app",
			Environment: "production",
			HasToken:    true,
			TokenHint:   "zen-***-123",
		})
	})
	defer server.Close()

	app, err := c.GetZenApp(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if app.Name != "my-app" {
		t.Errorf("expected name 'my-app', got %q", app.Name)
	}
	if !app.HasToken {
		t.Error("expected has_token true")
	}
}

func TestGetZenApp_NotFound(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		mustEncode(t, w, map[string]string{"reason_phrase": "not found"})
	})
	defer server.Close()

	_, err := c.GetZenApp(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error for not found")
	}
}

func TestListZenApps(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		mustEncode(t, w, []ZenApp{
			{ID: "1", Name: "app-a", Environment: "production", Blocking: true},
			{ID: "2", Name: "app-b", Environment: "staging", Blocking: false},
		})
	})
	defer server.Close()

	apps, err := c.ListZenApps(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(apps) != 2 {
		t.Errorf("expected 2 apps, got %d", len(apps))
	}
}

func TestUpdateZenApp(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/api/public/v1/firewall/apps/42" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var req UpdateZenAppRequest
		mustDecode(t, r, &req)
		if req.Name != "updated-app" {
			t.Errorf("expected name 'updated-app', got %q", req.Name)
		}
		mustEncode(t, w, map[string]bool{"success": true})
	})
	defer server.Close()

	err := c.UpdateZenApp(context.Background(), 42, UpdateZenAppRequest{
		Name:        "updated-app",
		Environment: "staging",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteZenApp(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/api/public/v1/firewall/apps/42" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		mustEncode(t, w, map[string]bool{"success": true})
	})
	defer server.Close()

	err := c.DeleteZenApp(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteZenApp_NotFound(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	err := c.DeleteZenApp(context.Background(), 999)
	if err != nil {
		t.Fatalf("expected no error for already-deleted app, got: %v", err)
	}
}
