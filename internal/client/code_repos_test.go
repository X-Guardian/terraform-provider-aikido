package client

import (
	"context"
	"net/http"
	"testing"
)

func TestGetCodeRepo(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/repositories/code/42" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		mustEncode(t, w, CodeRepoDetail{
			ID:           42,
			Name:         "my-repo",
			Provider:     "gitlab",
			Active:       true,
			Branch:       "main",
			Sensitivity:  "normal",
			Connectivity: "connected",
			ExcludedPaths: []ExcludedPath{
				{ID: 1, Path: "vendor/"},
			},
		})
	})
	defer server.Close()

	repo, err := c.GetCodeRepo(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.Name != "my-repo" {
		t.Errorf("expected name 'my-repo', got %q", repo.Name)
	}
	if len(repo.ExcludedPaths) != 1 {
		t.Errorf("expected 1 excluded path, got %d", len(repo.ExcludedPaths))
	}
}

func TestGetCodeRepo_NotFound(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	_, err := c.GetCodeRepo(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error for not found")
	}
}

func TestActivateCodeRepo(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/public/v1/repositories/code/activate" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var body map[string]int
		mustDecode(t, r, &body)
		if body["code_repo_id"] != 42 {
			t.Errorf("expected code_repo_id 42, got %d", body["code_repo_id"])
		}
		mustEncode(t, w, map[string]int{"success": 1})
	})
	defer server.Close()

	err := c.ActivateCodeRepo(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeactivateCodeRepo(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/repositories/code/deactivate" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		mustEncode(t, w, map[string]int{"success": 1})
	})
	defer server.Close()

	err := c.DeactivateCodeRepo(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateCodeRepoSensitivity(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/repositories/code/42/sensitivity" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		mustEncode(t, w, map[string]interface{}{"success": true})
	})
	defer server.Close()

	err := c.UpdateCodeRepoSensitivity(context.Background(), 42, "extreme")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateCodeRepoConnectivity(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/repositories/code/42/connectivity" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		mustEncode(t, w, map[string]interface{}{"success": true})
	})
	defer server.Close()

	err := c.UpdateCodeRepoConnectivity(context.Background(), 42, "connected")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateCodeRepoDevDepScanning(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/repositories/code/42/devdep-scan" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		mustEncode(t, w, map[string]interface{}{"success": 1})
	})
	defer server.Close()

	err := c.UpdateCodeRepoDevDepScanning(context.Background(), 42, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddCodeRepoExcludePath(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/repositories/code/42/exclude-path" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var body map[string]string
		mustDecode(t, r, &body)
		if body["path"] != "vendor/" {
			t.Errorf("expected path 'vendor/', got %q", body["path"])
		}
		mustEncode(t, w, map[string]int{"excluded_path_id": 1})
	})
	defer server.Close()

	err := c.AddCodeRepoExcludePath(context.Background(), 42, "vendor/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemoveCodeRepoExcludePath(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/repositories/code/42/exclude-path/remove" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		mustEncode(t, w, map[string]int{"success": 1})
	})
	defer server.Close()

	err := c.RemoveCodeRepoExcludePath(context.Background(), 42, "vendor/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
