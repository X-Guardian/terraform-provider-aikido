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

func TestAuthenticate_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/oauth/token" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.Header.Get("Authorization") == "" {
			t.Error("missing Authorization header")
		}
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("unexpected Content-Type: %s", r.Header.Get("Content-Type"))
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(tokenResponse{
			AccessToken: "test-token",
			ExpiresIn:   3600,
			TokenType:   "bearer",
		}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	c := NewAikidoClient(server.URL, "test-id", "test-secret")
	err := c.authenticate(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.accessToken != "test-token" {
		t.Errorf("expected token 'test-token', got %q", c.accessToken)
	}
}

func TestAuthenticate_BadCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte(`{"error":"invalid_client"}`)); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	c := NewAikidoClient(server.URL, "bad-id", "bad-secret")
	err := c.authenticate(context.Background())
	if err == nil {
		t.Fatal("expected error for bad credentials")
	}
}

func TestDoRequest_SetsAuthHeader(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/oauth/token" {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(tokenResponse{
				AccessToken: "my-bearer-token",
				ExpiresIn:   3600,
				TokenType:   "bearer",
			}); err != nil {
				t.Fatalf("failed to encode response: %v", err)
			}
			return
		}

		callCount++
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer my-bearer-token" {
			t.Errorf("expected 'Bearer my-bearer-token', got %q", authHeader)
		}
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`[]`)); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	c := NewAikidoClient(server.URL, "test-id", "test-secret")
	resp, err := c.DoRequest(context.Background(), http.MethodGet, "/teams", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	if callCount != 1 {
		t.Errorf("expected 1 API call, got %d", callCount)
	}
}
