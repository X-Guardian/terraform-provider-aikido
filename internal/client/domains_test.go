package client

import (
	"context"
	"net/http"
	"testing"
)

func TestCreateDomain(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/public/v1/domains" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var req CreateDomainRequest
		mustDecode(t, r, &req)
		if req.Domain != "example.com" {
			t.Errorf("expected domain 'example.com', got %q", req.Domain)
		}
		if req.Kind != "front_end" {
			t.Errorf("expected kind 'front_end', got %q", req.Kind)
		}
		w.WriteHeader(http.StatusCreated)
		mustEncode(t, w, CreateDomainResponse{ID: 42})
	})
	defer server.Close()

	id, err := c.CreateDomain(context.Background(), CreateDomainRequest{
		Domain: "example.com",
		Kind:   "front_end",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 42 {
		t.Errorf("expected ID 42, got %d", id)
	}
}

func TestGetDomain_Found(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		mustEncode(t, w, []Domain{
			{ID: 1, Domain: "foo.com", Kind: "front_end"},
			{ID: 2, Domain: "api.foo.com", Kind: "rest_api"},
		})
	})
	defer server.Close()

	domain, err := c.GetDomain(context.Background(), 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if domain.Domain != "api.foo.com" {
		t.Errorf("expected domain 'api.foo.com', got %q", domain.Domain)
	}
}

func TestGetDomain_NotFound(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		mustEncode(t, w, []Domain{})
	})
	defer server.Close()

	_, err := c.GetDomain(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error for not found")
	}
}

func TestListDomains(t *testing.T) {
	page := 0
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		switch page {
		case 0:
			mustEncode(t, w, []Domain{
				{ID: 1, Domain: "example.com", Kind: "front_end"},
				{ID: 2, Domain: "api.example.com", Kind: "rest_api"},
			})
			page++
		default:
			mustEncode(t, w, []Domain{})
		}
	})
	defer server.Close()

	domains, err := c.ListDomains(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(domains) != 2 {
		t.Errorf("expected 2 domains, got %d", len(domains))
	}
}

func TestDeleteDomain_Success(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/api/public/v1/domains/42" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		mustEncode(t, w, map[string]bool{"status": true})
	})
	defer server.Close()

	err := c.DeleteDomain(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteDomain_NotFound(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	err := c.DeleteDomain(context.Background(), 999)
	if err != nil {
		t.Fatalf("expected no error for already-deleted domain, got: %v", err)
	}
}
