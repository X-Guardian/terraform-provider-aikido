package client

import (
	"context"
	"net/http"
	"testing"
)

func TestLinkResourceToTeam(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/public/v1/teams/10/linkResource" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var body map[string]int
		mustDecode(t, r, &body)
		if body["repo_id"] != 42 {
			t.Errorf("expected repo_id 42, got %d", body["repo_id"])
		}
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	err := c.LinkResourceToTeam(context.Background(), 10, "code_repository", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLinkResourceToTeam_Cloud(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]int
		mustDecode(t, r, &body)
		if body["cloud_id"] != 99 {
			t.Errorf("expected cloud_id 99, got %d", body["cloud_id"])
		}
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	err := c.LinkResourceToTeam(context.Background(), 10, "cloud", 99)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLinkResourceToTeam_UnknownType(t *testing.T) {
	err := (&AikidoClient{}).LinkResourceToTeam(context.Background(), 10, "unknown_type", 42)
	if err == nil {
		t.Fatal("expected error for unknown resource type")
	}
}

func TestUnlinkResourceFromTeam(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/public/v1/teams/10/unlinkResource" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var body map[string]int
		mustDecode(t, r, &body)
		if body["repo_id"] != 42 {
			t.Errorf("expected repo_id 42, got %d", body["repo_id"])
		}
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	err := c.UnlinkResourceFromTeam(context.Background(), 10, "code_repository", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIsResourceLinkedToTeam_Found(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		mustEncode(t, w, []Team{
			{
				ID:   10,
				Name: "test-team",
				Responsibilities: []Responsibility{
					{ID: 42, Type: "code_repository"},
					{ID: 99, Type: "cloud"},
				},
			},
		})
	})
	defer server.Close()

	found, err := c.IsResourceLinkedToTeam(context.Background(), 10, "code_repository", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Error("expected resource to be linked to team")
	}
}

func TestIsResourceLinkedToTeam_NotFound(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		mustEncode(t, w, []Team{
			{
				ID:   10,
				Name: "test-team",
				Responsibilities: []Responsibility{
					{ID: 99, Type: "cloud"},
				},
			},
		})
	})
	defer server.Close()

	found, err := c.IsResourceLinkedToTeam(context.Background(), 10, "code_repository", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Error("expected resource NOT to be linked to team")
	}
}
