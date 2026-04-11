// Copyright IBM Corp. 2025
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestServer(handler http.HandlerFunc) (*httptest.Server, *AikidoClient) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/oauth/token" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(tokenResponse{
				AccessToken: "test-token",
				ExpiresIn:   3600,
				TokenType:   "bearer",
			})
			return
		}
		handler(w, r)
	}))
	client := NewAikidoClient(server.URL, "test-id", "test-secret")
	client.SetRateLimit(1000) // Effectively disable rate limiting in tests
	return server, client
}

func mustEncode(t *testing.T, w http.ResponseWriter, v interface{}) {
	t.Helper()
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("failed to encode response: %v", err)
	}
}

func mustDecode(t *testing.T, r *http.Request, v interface{}) {
	t.Helper()
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		t.Fatalf("failed to decode request: %v", err)
	}
}

func TestCreateTeam(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/public/v1/teams":
			var req CreateTeamRequest
			mustDecode(t, r, &req)
			if req.Name != "test-team" {
				t.Errorf("expected name 'test-team', got %q", req.Name)
			}
			w.WriteHeader(http.StatusCreated)
			mustEncode(t, w, map[string]interface{}{"id": 42})

		case r.Method == http.MethodGet && r.URL.Path == "/api/public/v1/teams":
			mustEncode(t, w, []Team{
				{ID: 42, Name: "test-team", Active: true},
			})

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer server.Close()

	team, err := c.CreateTeam(context.Background(), "test-team")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if team.ID != 42 {
		t.Errorf("expected ID 42, got %d", team.ID)
	}
	if team.Name != "test-team" {
		t.Errorf("expected name 'test-team', got %q", team.Name)
	}
}

func TestGetTeam_Found(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		mustEncode(t, w, []Team{
			{ID: 1, Name: "team-a"},
			{ID: 2, Name: "team-b"},
		})
	})
	defer server.Close()

	team, err := c.GetTeam(context.Background(), 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if team.Name != "team-b" {
		t.Errorf("expected name 'team-b', got %q", team.Name)
	}
}

func TestGetTeam_NotFound(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		mustEncode(t, w, []Team{})
	})
	defer server.Close()

	_, err := c.GetTeam(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error for team not found")
	}
}

func TestGetTeam_Pagination(t *testing.T) {
	page := 0
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		switch page {
		case 0:
			teams := make([]Team, 20)
			for i := range teams {
				teams[i] = Team{ID: i + 1, Name: "team"}
			}
			mustEncode(t, w, teams)
			page++
		case 1:
			mustEncode(t, w, []Team{
				{ID: 21, Name: "target-team"},
			})
			page++
		default:
			mustEncode(t, w, []Team{})
		}
	})
	defer server.Close()

	team, err := c.GetTeam(context.Background(), 21)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if team.Name != "target-team" {
		t.Errorf("expected name 'target-team', got %q", team.Name)
	}
}

func TestUpdateTeam(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/api/public/v1/teams/42" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var req UpdateTeamRequest
		mustDecode(t, r, &req)
		if req.Name != "updated-name" {
			t.Errorf("expected name 'updated-name', got %q", req.Name)
		}
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	err := c.UpdateTeam(context.Background(), 42, UpdateTeamRequest{Name: "updated-name"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteTeam_Success(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	defer server.Close()

	err := c.DeleteTeam(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteTeam_ImportedTeam(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte(`{"error":"Imported teams cannot be deleted."}`)); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	})
	defer server.Close()

	err := c.DeleteTeam(context.Background(), 42)
	if err == nil {
		t.Fatal("expected error for imported team deletion")
	}
}

func TestListTeams(t *testing.T) {
	page := 0
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		switch page {
		case 0:
			mustEncode(t, w, []Team{
				{ID: 1, Name: "team-a"},
				{ID: 2, Name: "team-b"},
			})
			page++
		default:
			mustEncode(t, w, []Team{})
		}
	})
	defer server.Close()

	teams, err := c.ListTeams(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(teams) != 2 {
		t.Errorf("expected 2 teams, got %d", len(teams))
	}
}
