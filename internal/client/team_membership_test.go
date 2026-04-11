// Copyright IBM Corp. 2025
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"net/http"
	"testing"
)

func TestAddUserToTeam(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/public/v1/teams/10/addUser" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var body map[string]int
		mustDecode(t, r, &body)
		if body["user_id"] != 5 {
			t.Errorf("expected user_id 5, got %d", body["user_id"])
		}
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	err := c.AddUserToTeam(context.Background(), 10, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddUserToTeam_NotFound(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		if _, err := w.Write([]byte(`{"error":"The team or user has not been found"}`)); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	})
	defer server.Close()

	err := c.AddUserToTeam(context.Background(), 999, 999)
	if err == nil {
		t.Fatal("expected error for not found")
	}
}

func TestRemoveUserFromTeam(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/public/v1/teams/10/removeUser" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	err := c.RemoveUserFromTeam(context.Background(), 10, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIsUserInTeam_Found(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/users" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("filter_team_id") != "10" {
			t.Errorf("expected filter_team_id=10, got %s", r.URL.Query().Get("filter_team_id"))
		}
		mustEncode(t, w, []User{
			{ID: 3},
			{ID: 5},
			{ID: 7},
		})
	})
	defer server.Close()

	found, err := c.IsUserInTeam(context.Background(), 10, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Error("expected user to be found in team")
	}
}

func TestIsUserInTeam_NotFound(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		mustEncode(t, w, []User{
			{ID: 3},
			{ID: 7},
		})
	})
	defer server.Close()

	found, err := c.IsUserInTeam(context.Background(), 10, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Error("expected user NOT to be found in team")
	}
}
