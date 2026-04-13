package client

import (
	"context"
	"net/http"
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
