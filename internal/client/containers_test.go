package client

import (
	"context"
	"net/http"
	"testing"
)

func TestGetContainer(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/containers/42" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		mustEncode(t, w, Container{
			ID:       42,
			Name:     "my-app",
			Provider: "aws",
			Tag:      "latest",
			Distro:   "alpine",
		})
	})
	defer server.Close()

	container, err := c.GetContainer(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if container.Name != "my-app" {
		t.Errorf("expected name 'my-app', got %q", container.Name)
	}
}

func TestGetContainer_NotFound(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	_, err := c.GetContainer(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error for not found")
	}
}

func TestListContainers(t *testing.T) {
	page := 0
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		switch page {
		case 0:
			mustEncode(t, w, []Container{
				{ID: 1, Name: "app-a"},
				{ID: 2, Name: "app-b"},
			})
			page++
		default:
			mustEncode(t, w, []Container{})
		}
	})
	defer server.Close()

	containers, err := c.ListContainers(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(containers) != 2 {
		t.Errorf("expected 2 containers, got %d", len(containers))
	}
}

func TestActivateContainer(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/containers/activate" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var body map[string]int
		mustDecode(t, r, &body)
		if body["container_repo_id"] != 42 {
			t.Errorf("expected container_repo_id 42, got %d", body["container_repo_id"])
		}
		mustEncode(t, w, map[string]int{"success": 1})
	})
	defer server.Close()

	err := c.ActivateContainer(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeactivateContainer(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/containers/deactivate" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		mustEncode(t, w, map[string]int{"success": 1})
	})
	defer server.Close()

	err := c.DeactivateContainer(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateContainerSensitivity(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/containers/42/sensitivity" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		mustEncode(t, w, map[string]interface{}{"success": true})
	})
	defer server.Close()

	err := c.UpdateContainerSensitivity(context.Background(), 42, "extreme")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateContainerConnectivity(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/containers/42/internetConnection" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		mustEncode(t, w, map[string]interface{}{"success": true})
	})
	defer server.Close()

	err := c.UpdateContainerConnectivity(context.Background(), 42, "connected")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateContainerTagFilter(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/containers/updateTagFilter" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		mustEncode(t, w, map[string]int{"success": 1})
	})
	defer server.Close()

	err := c.UpdateContainerTagFilter(context.Background(), 42, "v*")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
