package client

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func TestGetCloud_Found(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		mustEncode(t, w, []Cloud{
			{ID: 1, Name: "aws-prod", Provider: "aws", Environment: "production", ExternalID: "123456"},
			{ID: 2, Name: "gcp-staging", Provider: "gcp", Environment: "staging", ExternalID: "my-project"},
		})
	})
	defer server.Close()

	cloud, err := c.GetCloud(context.Background(), 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cloud.Name != "gcp-staging" {
		t.Errorf("expected name 'gcp-staging', got %q", cloud.Name)
	}
}

func TestGetCloud_NotFound(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		mustEncode(t, w, []Cloud{})
	})
	defer server.Close()

	_, err := c.GetCloud(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error for cloud not found")
	}
}

func TestGetCloud_PermissionError(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		if _, err := w.Write([]byte(`{"error":"You are missing the required scope for this request: \u0027clouds:read\u0027"}`)); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	})
	defer server.Close()

	_, err := c.GetCloud(context.Background(), 8206)
	if err == nil {
		t.Fatal("expected error for 403")
	}
	// Verify the error message is clean (no unicode escapes)
	if !strings.Contains(err.Error(), "'clouds:read'") {
		t.Errorf("expected clean error with 'clouds:read', got: %s", err.Error())
	}
	// Verify it's not a "not found" error (important for Read method distinction)
	if strings.Contains(err.Error(), "not found") {
		t.Error("403 error should not contain 'not found'")
	}
}

func TestCreateAWSCloud(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/public/v1/clouds/aws" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var req CreateAWSCloudRequest
		mustDecode(t, r, &req)
		if req.Name != "test-aws" {
			t.Errorf("expected name 'test-aws', got %q", req.Name)
		}
		if req.RoleARN != "arn:aws:iam::123:role/test" {
			t.Errorf("unexpected role_arn: %s", req.RoleARN)
		}
		w.WriteHeader(http.StatusCreated)
		mustEncode(t, w, CreateCloudResponse{ID: 42})
	})
	defer server.Close()

	id, err := c.CreateAWSCloud(context.Background(), CreateAWSCloudRequest{
		Name:        "test-aws",
		Environment: "production",
		RoleARN:     "arn:aws:iam::123:role/test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 42 {
		t.Errorf("expected ID 42, got %d", id)
	}
}

func TestCreateAzureCloud(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/clouds/azure" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		mustEncode(t, w, CreateCloudResponse{ID: 43})
	})
	defer server.Close()

	id, err := c.CreateAzureCloud(context.Background(), CreateAzureCloudRequest{
		Name:           "test-azure",
		Environment:    "staging",
		ApplicationID:  "app-id",
		DirectoryID:    "dir-id",
		SubscriptionID: "sub-id",
		KeyValue:       "secret",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 43 {
		t.Errorf("expected ID 43, got %d", id)
	}
}

func TestCreateGCPCloud(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/clouds/gcp" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		mustEncode(t, w, CreateCloudResponse{ID: 44})
	})
	defer server.Close()

	id, err := c.CreateGCPCloud(context.Background(), CreateGCPCloudRequest{
		Name:        "test-gcp",
		Environment: "development",
		ProjectID:   "my-project",
		AccessKey:   `{"type":"service_account"}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 44 {
		t.Errorf("expected ID 44, got %d", id)
	}
}

func TestCreateKubernetesCloud(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/clouds/kubernetes" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		mustEncode(t, w, KubernetesCloudResponse{
			ID:         45,
			Endpoint:   "https://agent.aikido.dev",
			AgentToken: "token-123",
			CreatedAt:  1720000000,
		})
	})
	defer server.Close()

	resp, err := c.CreateKubernetesCloud(context.Background(), CreateKubernetesCloudRequest{
		Name:                "test-k8s",
		Environment:         "production",
		ExcludedNamespaces:  []string{"kube-system"},
		EnableImageScanning: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != 45 {
		t.Errorf("expected ID 45, got %d", resp.ID)
	}
	if resp.AgentToken != "token-123" {
		t.Errorf("expected agent_token 'token-123', got %q", resp.AgentToken)
	}
}

func TestDeleteCloud_Success(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/api/public/v1/clouds/42" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		mustEncode(t, w, map[string]bool{"success": true})
	})
	defer server.Close()

	err := c.DeleteCloud(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteCloud_NotFound(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		mustEncode(t, w, map[string]string{"reason_phrase": "Cloud not found"})
	})
	defer server.Close()

	err := c.DeleteCloud(context.Background(), 999)
	if err != nil {
		t.Fatalf("expected no error for already-deleted cloud, got: %v", err)
	}
}

func TestGetCloud_RequestsMaxPageSize(t *testing.T) {
	var perPage string
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		perPage = r.URL.Query().Get("per_page")
		mustEncode(t, w, []Cloud{{ID: 1, Name: "aws-prod"}})
	})
	defer server.Close()

	if _, err := c.GetCloud(context.Background(), 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if perPage != "100" {
		t.Errorf("expected per_page=100, got %q", perPage)
	}
}

func TestGetCloud_ServesRepeatedReadsFromCache(t *testing.T) {
	var calls atomic.Int32
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		mustEncode(t, w, []Cloud{{ID: 1, Name: "one"}, {ID: 2, Name: "two"}})
	})
	defer server.Close()

	for _, id := range []int{1, 2, 1, 2} {
		if _, err := c.GetCloud(context.Background(), id); err != nil {
			t.Fatalf("unexpected error for %d: %v", id, err)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("expected 1 list request for 4 reads, got %d", got)
	}
}

func TestGetCloud_RefetchesOnceOnMiss(t *testing.T) {
	var calls atomic.Int32
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		clouds := []Cloud{{ID: 1, Name: "one"}}
		if n > 1 {
			clouds = append(clouds, Cloud{ID: 2, Name: "created-after-first-list"})
		}
		mustEncode(t, w, clouds)
	})
	defer server.Close()

	if _, err := c.GetCloud(context.Background(), 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cloud, err := c.GetCloud(context.Background(), 2)
	if err != nil {
		t.Fatalf("expected refetch to find cloud 2, got error: %v", err)
	}
	if cloud.Name != "created-after-first-list" {
		t.Errorf("unexpected cloud: %+v", cloud)
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("expected exactly 2 list requests (initial + one refetch), got %d", got)
	}
}

func TestCreateAWSCloud_InvalidatesCloudsCache(t *testing.T) {
	var lists atomic.Int32
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			mustEncode(t, w, map[string]int{"id": 7})
			return
		}
		n := lists.Add(1)
		clouds := []Cloud{{ID: 1, Name: "one"}}
		if n > 1 {
			clouds = append(clouds, Cloud{ID: 7, Name: "new"})
		}
		mustEncode(t, w, clouds)
	})
	defer server.Close()

	if _, err := c.ListClouds(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	id, err := c.CreateAWSCloud(context.Background(), CreateAWSCloudRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cloud, err := c.GetCloud(context.Background(), id)
	if err != nil {
		t.Fatalf("expected the created cloud to be readable, got: %v", err)
	}
	if cloud.Name != "new" {
		t.Errorf("unexpected cloud: %+v", cloud)
	}
	if got := lists.Load(); got != 2 {
		t.Errorf("expected 2 list requests (before and after create), got %d", got)
	}
}
