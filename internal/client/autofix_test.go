package client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestGetAutofixDependencySettings(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/public/v1/repositories/autofix/dependency/settings" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		mustEncode(t, w, map[string]interface{}{
			"settings": map[string]interface{}{
				"enabled":                      true,
				"severity_filter":              "upgrade_all_packages",
				"repos_scope":                  "selected",
				"repo_ids":                     []int{123},
				"use_aikido_library_for_major": false,
			},
		})
	})
	defer server.Close()

	settings, err := c.GetAutofixDependencySettings(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !settings.Enabled {
		t.Error("expected enabled to be true")
	}
	if settings.SeverityFilter != "upgrade_all_packages" {
		t.Errorf("expected severity_filter 'upgrade_all_packages', got %q", settings.SeverityFilter)
	}
	if settings.ReposScope != "selected" {
		t.Errorf("expected repos_scope 'selected', got %q", settings.ReposScope)
	}
	if len(settings.RepoIDs) != 1 || settings.RepoIDs[0] != 123 {
		t.Errorf("expected repo_ids [123], got %v", settings.RepoIDs)
	}
	if settings.UseAikidoLibraryForMajor {
		t.Error("expected use_aikido_library_for_major to be false")
	}
}

// TestGetAutofixDependencySettings_SeverityNone covers the read-only "none" value, which
// the GET returns when autofix is disabled but the PUT enum does not accept.
func TestGetAutofixDependencySettings_SeverityNone(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		mustEncode(t, w, map[string]interface{}{
			"settings": map[string]interface{}{
				"enabled":                      false,
				"severity_filter":              "none",
				"repos_scope":                  "all",
				"repo_ids":                     []int{},
				"use_aikido_library_for_major": false,
			},
		})
	})
	defer server.Close()

	settings, err := c.GetAutofixDependencySettings(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if settings.Enabled {
		t.Error("expected enabled to be false")
	}
	if settings.SeverityFilter != "none" {
		t.Errorf("expected severity_filter 'none', got %q", settings.SeverityFilter)
	}
}

func TestUpdateAutofixDependencySettings_Enabled(t *testing.T) {
	var body map[string]interface{}
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/public/v1/repositories/autofix/dependency/settings" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		mustDecode(t, r, &body)
		mustEncode(t, w, map[string]int{"success": 1})
	})
	defer server.Close()

	severity := "critical_and_high_only"
	scope := "selected"
	repoIDs := []int64{7, 9}
	useLibrary := true
	err := c.UpdateAutofixDependencySettings(context.Background(), AutofixSettingsRequest{
		Enabled:                  true,
		SeverityFilter:           &severity,
		ReposScope:               &scope,
		RepoIDs:                  &repoIDs,
		UseAikidoLibraryForMajor: &useLibrary,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if body["enabled"] != true {
		t.Errorf("expected enabled true, got %v", body["enabled"])
	}
	if body["severity_filter"] != "critical_and_high_only" {
		t.Errorf("expected severity_filter 'critical_and_high_only', got %v", body["severity_filter"])
	}
	if body["repos_scope"] != "selected" {
		t.Errorf("expected repos_scope 'selected', got %v", body["repos_scope"])
	}
	if body["use_aikido_library_for_major"] != true {
		t.Errorf("expected use_aikido_library_for_major true, got %v", body["use_aikido_library_for_major"])
	}
	ids, ok := body["repo_ids"].([]interface{})
	if !ok || len(ids) != 2 {
		t.Fatalf("expected 2 repo_ids, got %v", body["repo_ids"])
	}
}

// TestUpdateAutofixDependencySettings_Disabled asserts the disable payload omits every
// field the API ignores, matching the "Disable" variant of the endpoint's oneOf schema.
func TestUpdateAutofixDependencySettings_Disabled(t *testing.T) {
	var body map[string]interface{}
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		mustDecode(t, r, &body)
		mustEncode(t, w, map[string]int{"success": 1})
	})
	defer server.Close()

	if err := c.UpdateAutofixDependencySettings(context.Background(), AutofixSettingsRequest{Enabled: false}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if body["enabled"] != false {
		t.Errorf("expected enabled false, got %v", body["enabled"])
	}
	if len(body) != 1 {
		t.Errorf("expected only the enabled key, got %v", body)
	}
}

// TestUpdateAutofixSettings_ScopeAllSendsEmptyRepoIDs asserts that repos_scope "all"
// sends an explicit empty array rather than omitting the key, which is what the pointer
// field on AutofixSettingsRequest exists to make possible.
func TestUpdateAutofixSettings_ScopeAllSendsEmptyRepoIDs(t *testing.T) {
	var body map[string]interface{}
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		mustDecode(t, r, &body)
		mustEncode(t, w, map[string]int{"success": 1})
	})
	defer server.Close()

	severity := "all"
	scope := "all"
	repoIDs := []int64{}
	if err := c.UpdateAutofixSastSettings(context.Background(), AutofixSettingsRequest{
		Enabled:        true,
		SeverityFilter: &severity,
		ReposScope:     &scope,
		RepoIDs:        &repoIDs,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ids, ok := body["repo_ids"]
	if !ok {
		t.Fatal("expected repo_ids key to be present")
	}
	if list, ok := ids.([]interface{}); !ok || len(list) != 0 {
		t.Errorf("expected empty repo_ids array, got %v", ids)
	}
	// use_aikido_library_for_major is dependency-only and must not leak into the SAST payload.
	if _, present := body["use_aikido_library_for_major"]; present {
		t.Error("did not expect use_aikido_library_for_major in the SAST payload")
	}
}

func TestGetAutofixSastSettings(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/repositories/autofix/sast/settings" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		mustEncode(t, w, map[string]interface{}{
			"settings": map[string]interface{}{
				"enabled":         true,
				"severity_filter": "critical_and_high_only",
				"repos_scope":     "all",
				"repo_ids":        []int{},
			},
		})
	})
	defer server.Close()

	settings, err := c.GetAutofixSastSettings(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if settings.SeverityFilter != "critical_and_high_only" {
		t.Errorf("expected severity_filter 'critical_and_high_only', got %q", settings.SeverityFilter)
	}
	if settings.ReposScope != "all" {
		t.Errorf("expected repos_scope 'all', got %q", settings.ReposScope)
	}
	if len(settings.RepoIDs) != 0 {
		t.Errorf("expected no repo_ids, got %v", settings.RepoIDs)
	}
}

func TestGetAutofixPentestSettings(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/v1/repositories/autofix/pentest/settings" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		mustEncode(t, w, map[string]interface{}{
			"settings": map[string]interface{}{
				"enabled":         true,
				"severity_filter": "all",
			},
		})
	})
	defer server.Close()

	settings, err := c.GetAutofixPentestSettings(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !settings.Enabled {
		t.Error("expected enabled to be true")
	}
	if settings.SeverityFilter != "all" {
		t.Errorf("expected severity_filter 'all', got %q", settings.SeverityFilter)
	}
}

func TestUpdateAutofixPentestSettings(t *testing.T) {
	var body map[string]interface{}
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/public/v1/repositories/autofix/pentest/settings" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		mustDecode(t, r, &body)
		mustEncode(t, w, map[string]int{"success": 1})
	})
	defer server.Close()

	severity := "critical_and_high_only"
	if err := c.UpdateAutofixPentestSettings(context.Background(), AutofixPentestSettingsRequest{
		Enabled:        true,
		SeverityFilter: &severity,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if body["severity_filter"] != "critical_and_high_only" {
		t.Errorf("expected severity_filter 'critical_and_high_only', got %v", body["severity_filter"])
	}
	if _, present := body["repos_scope"]; present {
		t.Error("did not expect repos_scope in the pentest payload")
	}
}

// TestGetAutofixSettings_WorkspaceDisabled covers the 400 the endpoints return when
// AutoFix is switched off for the whole workspace, which must be detectable by callers.
func TestGetAutofixSettings_WorkspaceDisabled(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		mustEncode(t, w, map[string]string{"reason_phrase": "AutoFix has been disabled for this workspace."})
	})
	defer server.Close()

	_, err := c.GetAutofixSastSettings(context.Background())
	if err == nil {
		t.Fatal("expected an error")
	}
	if !errors.Is(err, ErrAutofixDisabledForWorkspace) {
		t.Errorf("expected ErrAutofixDisabledForWorkspace, got %v", err)
	}
	if !strings.Contains(err.Error(), "AutoFix has been disabled for this workspace.") {
		t.Errorf("expected the reason phrase in the message, got %v", err)
	}
}

// TestUpdateAutofixSettings_WorkspaceDisabled covers the same condition on a write. The
// PUT 400 has no JSON body, so errorBody falls through to the raw text.
func TestUpdateAutofixSettings_WorkspaceDisabled(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, "AutoFix has been disabled for this workspace.")
	})
	defer server.Close()

	err := c.UpdateAutofixPentestSettings(context.Background(), AutofixPentestSettingsRequest{Enabled: false})
	if !errors.Is(err, ErrAutofixDisabledForWorkspace) {
		t.Errorf("expected ErrAutofixDisabledForWorkspace, got %v", err)
	}
}

// TestGetAutofixSettings_OtherError checks that unrelated failures are not mistaken for
// workspace-level disablement.
func TestGetAutofixSettings_OtherError(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		mustEncode(t, w, map[string]string{"error": "You are missing the required scope for this request: 'autofix:read'"})
	})
	defer server.Close()

	_, err := c.GetAutofixDependencySettings(context.Background())
	if err == nil {
		t.Fatal("expected an error")
	}
	if errors.Is(err, ErrAutofixDisabledForWorkspace) {
		t.Error("a 403 scope error must not be reported as workspace disablement")
	}
	if !strings.Contains(err.Error(), "autofix:read") {
		t.Errorf("expected the scope message to be surfaced, got %v", err)
	}
}
