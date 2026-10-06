package client

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

// liveCIChecksResponse mirrors a real response from the API, including its omission of
// post_deep_audit_inline_comments_min_severity, so the tests pin actual API behaviour rather than what the spec
// declares.
const liveCIChecksResponse = `[
  {
    "id": 237367,
    "code_repo_id": 410167,
    "minimum_severity": "high",
    "fail_on_dependency_scan": true,
    "fail_on_sast_scan": true,
    "fail_on_iac_scan": true,
    "fail_on_secrets_scan": true,
    "fail_on_malware_scan": true,
    "post_inline_comments_min_severity": "high",
    "minimum_license_severity": "critical",
    "fail_on_code_quality_scan": false,
    "enable_code_quality_scan": true,
    "post_code_quality_inline_comments_min_severity": "high",
    "run_deep_audit_pr_scan": false
  }
]`

func TestGetCIChecksConfiguration(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/public/v1/repositories/code/continuous_integration/checks" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		q := r.URL.Query()
		if got := q.Get("filter_code_repo_id"); got != "410167" {
			t.Errorf("filter_code_repo_id = %q, want %q", got, "410167")
		}
		if got := q.Get("page"); got != "0" {
			t.Errorf("page = %q, want %q", got, "0")
		}
		if got := q.Get("per_page"); got != "100" {
			t.Errorf("per_page = %q, want %q", got, "100")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(liveCIChecksResponse))
	})
	defer server.Close()

	config, err := c.GetCIChecksConfiguration(context.Background(), 410167)
	if err != nil {
		t.Fatalf("GetCIChecksConfiguration() error = %v", err)
	}

	if config.ID != 237367 {
		t.Errorf("ID = %d, want 237367", config.ID)
	}
	if config.CodeRepoID != 410167 {
		t.Errorf("CodeRepoID = %d, want 410167", config.CodeRepoID)
	}
	if config.MinimumSeverity != "high" {
		t.Errorf("MinimumSeverity = %q, want %q", config.MinimumSeverity, "high")
	}
	if config.MinimumLicenseSeverity != "critical" {
		t.Errorf("MinimumLicenseSeverity = %q, want %q", config.MinimumLicenseSeverity, "critical")
	}
	if !config.FailOnDependencyScan || !config.FailOnSastScan || !config.FailOnIacScan ||
		!config.FailOnSecretsScan || !config.FailOnMalwareScan {
		t.Error("expected every fail_on_*_scan to be true")
	}
	if config.FailOnCodeQualityScan {
		t.Error("FailOnCodeQualityScan = true, want false")
	}
	if !config.EnableCodeQualityScan {
		t.Error("EnableCodeQualityScan = false, want true")
	}
	if config.PostInlineCommentsMinSeverity != "high" {
		t.Errorf("PostInlineCommentsMinSeverity = %q, want %q", config.PostInlineCommentsMinSeverity, "high")
	}
	if config.PostCodeQualityInlineCommentsMinSeverity == nil {
		t.Fatal("PostCodeQualityInlineCommentsMinSeverity = nil, want \"high\"")
	}
	if *config.PostCodeQualityInlineCommentsMinSeverity != "high" {
		t.Errorf("PostCodeQualityInlineCommentsMinSeverity = %q, want %q",
			*config.PostCodeQualityInlineCommentsMinSeverity, "high")
	}
	if config.RunDeepAuditPRScan {
		t.Error("RunDeepAuditPRScan = true, want false")
	}
}

func TestGetCIChecksConfiguration_NullCodeQualitySeverity(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		mustEncode(t, w, []map[string]interface{}{{
			"id":                       1,
			"code_repo_id":             42,
			"minimum_severity":         "low",
			"enable_code_quality_scan": false,
			"post_code_quality_inline_comments_min_severity": nil,
		}})
	})
	defer server.Close()

	config, err := c.GetCIChecksConfiguration(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetCIChecksConfiguration() error = %v", err)
	}

	if config.PostCodeQualityInlineCommentsMinSeverity != nil {
		t.Errorf("PostCodeQualityInlineCommentsMinSeverity = %q, want nil",
			*config.PostCodeQualityInlineCommentsMinSeverity)
	}
}

func TestGetCIChecksConfiguration_EmptyArray(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		mustEncode(t, w, []CIChecksConfiguration{})
	})
	defer server.Close()

	_, err := c.GetCIChecksConfiguration(context.Background(), 42)
	if !errors.Is(err, ErrCIChecksNotFound) {
		t.Errorf("error = %v, want ErrCIChecksNotFound", err)
	}
}

func TestGetCIChecksConfiguration_NotFoundStatus(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	_, err := c.GetCIChecksConfiguration(context.Background(), 42)
	if !errors.Is(err, ErrCIChecksNotFound) {
		t.Errorf("error = %v, want ErrCIChecksNotFound", err)
	}
}

// TestGetCIChecksConfiguration_FilterIgnored pins the defensive pagination: a server that ignores
// filter_code_repo_id still yields the right configuration rather than the wrong one.
func TestGetCIChecksConfiguration_FilterIgnored(t *testing.T) {
	var pagesServed []string
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		pagesServed = append(pagesServed, page)

		if page == "0" {
			// A full page of other repositories' configurations forces a second request.
			configs := make([]CIChecksConfiguration, ciChecksPageSize)
			for i := range configs {
				configs[i] = CIChecksConfiguration{ID: i + 1, CodeRepoID: 900000 + i}
			}
			mustEncode(t, w, configs)
			return
		}
		mustEncode(t, w, []CIChecksConfiguration{{ID: 555, CodeRepoID: 42, MinimumSeverity: "critical"}})
	})
	defer server.Close()

	config, err := c.GetCIChecksConfiguration(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetCIChecksConfiguration() error = %v", err)
	}
	if config.ID != 555 || config.CodeRepoID != 42 {
		t.Errorf("got ID=%d CodeRepoID=%d, want ID=555 CodeRepoID=42", config.ID, config.CodeRepoID)
	}
	if len(pagesServed) != 2 || pagesServed[0] != "0" || pagesServed[1] != "1" {
		t.Errorf("pages served = %v, want [0 1]", pagesServed)
	}
}

// TestGetCIChecksConfiguration_WrongRepoOnly ensures a server that ignores the filter cannot make the wrong
// repository's configuration look like the requested one.
func TestGetCIChecksConfiguration_WrongRepoOnly(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		mustEncode(t, w, []CIChecksConfiguration{{ID: 1, CodeRepoID: 999, MinimumSeverity: "low"}})
	})
	defer server.Close()

	_, err := c.GetCIChecksConfiguration(context.Background(), 42)
	if !errors.Is(err, ErrCIChecksNotFound) {
		t.Errorf("error = %v, want ErrCIChecksNotFound", err)
	}
}

func TestGetCIChecksConfiguration_Error(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		mustEncode(t, w, map[string]string{"error": "missing repositories:read scope"})
	})
	defer server.Close()

	_, err := c.GetCIChecksConfiguration(context.Background(), 42)
	if err == nil {
		t.Fatal("expected an error")
	}
	if errors.Is(err, ErrCIChecksNotFound) {
		t.Error("a 403 must not be reported as ErrCIChecksNotFound")
	}
	if got := err.Error(); !strings.Contains(got, "missing repositories:read scope") {
		t.Errorf("error = %q, want it to include the API message", got)
	}
}

func TestSaveCIChecksConfiguration(t *testing.T) {
	inlineSeverity := "critical"
	codeQualitySeverity := "high"
	runDeepAudit := true
	deepAuditSeverity := "medium"

	var body map[string]interface{}
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/public/v1/repositories/code/continuous_integration/checks" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		mustDecode(t, r, &body)
		mustEncode(t, w, map[string]int{"success": 1})
	})
	defer server.Close()

	err := c.SaveCIChecksConfiguration(context.Background(), SaveCIChecksConfigurationRequest{
		CodeRepoID:                               410167,
		MinimumSeverity:                          "high",
		FailOnDependencyScan:                     true,
		FailOnSastScan:                           true,
		FailOnIacScan:                            false,
		FailOnSecretsScan:                        true,
		FailOnMalwareScan:                        false,
		MinimumLicenseSeverity:                   "critical",
		EnableCodeQualityScan:                    true,
		FailOnCodeQualityScan:                    false,
		PostCodeQualityInlineCommentsMinSeverity: &codeQualitySeverity,
		PostInlineCommentsMinSeverity:            &inlineSeverity,
		RunDeepAuditPRScan:                       &runDeepAudit,
		PostDeepAuditInlineCommentsMinSeverity:   &deepAuditSeverity,
	})
	if err != nil {
		t.Fatalf("SaveCIChecksConfiguration() error = %v", err)
	}

	if got := body["code_repo_id"]; got != float64(410167) {
		t.Errorf("code_repo_id = %v, want 410167", got)
	}
	if got := body["minimum_severity"]; got != "high" {
		t.Errorf("minimum_severity = %v, want high", got)
	}
	if got := body["fail_on_iac_scan"]; got != false {
		t.Errorf("fail_on_iac_scan = %v, want false", got)
	}
	if got := body["post_inline_comments_min_severity"]; got != "critical" {
		t.Errorf("post_inline_comments_min_severity = %v, want critical", got)
	}
	if got := body["post_code_quality_inline_comments_min_severity"]; got != "high" {
		t.Errorf("post_code_quality_inline_comments_min_severity = %v, want high", got)
	}
	if got := body["run_deep_audit_pr_scan"]; got != true {
		t.Errorf("run_deep_audit_pr_scan = %v, want true", got)
	}
	if got := body["post_deep_audit_inline_comments_min_severity"]; got != "medium" {
		t.Errorf("post_deep_audit_inline_comments_min_severity = %v, want medium", got)
	}
}

// TestSaveCIChecksConfiguration_OptionalFieldsOmitted asserts the difference between the two pointer treatments: the
// optional fields disappear when nil, while the required-but-nullable code quality severity is still sent, as an
// explicit null.
func TestSaveCIChecksConfiguration_OptionalFieldsOmitted(t *testing.T) {
	var body map[string]interface{}
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		mustDecode(t, r, &body)
		mustEncode(t, w, map[string]int{"success": 1})
	})
	defer server.Close()

	err := c.SaveCIChecksConfiguration(context.Background(), SaveCIChecksConfigurationRequest{
		CodeRepoID:             42,
		MinimumSeverity:        "always_pass_check",
		MinimumLicenseSeverity: "none",
	})
	if err != nil {
		t.Fatalf("SaveCIChecksConfiguration() error = %v", err)
	}

	for _, key := range []string{
		"post_inline_comments_min_severity",
		"run_deep_audit_pr_scan",
		"post_deep_audit_inline_comments_min_severity",
	} {
		if _, present := body[key]; present {
			t.Errorf("%s must be omitted when nil, got %v", key, body[key])
		}
	}

	// The API requires the key to be present even when the value is null, so a nil pointer must still produce the key.
	// This is why the field has no omitempty.
	value, present := body["post_code_quality_inline_comments_min_severity"]
	if !present {
		t.Error("post_code_quality_inline_comments_min_severity must always be sent, even when null")
	}
	if value != nil {
		t.Errorf("post_code_quality_inline_comments_min_severity = %v, want null", value)
	}
}

func TestGetCIChecksDefaultConfiguration(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/public/v1/repositories/code/continuous_integration/checks/default" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		mustEncode(t, w, map[string]interface{}{
			"is_enabled":                                     true,
			"minimum_severity":                               "high",
			"fail_on_dependency_scan":                        true,
			"fail_on_sast_scan":                              false,
			"fail_on_iac_scan":                               true,
			"fail_on_secrets_scan":                           true,
			"fail_on_malware_scan":                           true,
			"fail_on_license_scan":                           true,
			"minimum_license_severity":                       "critical",
			"post_inline_comments":                           false,
			"post_inline_comments_min_severity":              nil,
			"enable_code_quality_scan":                       false,
			"post_code_quality_inline_comments_min_severity": "none",
			"fail_on_code_quality_scan":                      false,
			"run_deep_audit_pr_scan":                         true,
		})
	})
	defer server.Close()

	config, err := c.GetCIChecksDefaultConfiguration(context.Background())
	if err != nil {
		t.Fatalf("GetCIChecksDefaultConfiguration() error = %v", err)
	}

	if !config.IsEnabled {
		t.Error("IsEnabled = false, want true")
	}
	if config.MinimumSeverity != "high" {
		t.Errorf("MinimumSeverity = %q, want %q", config.MinimumSeverity, "high")
	}
	if config.FailOnSastScan {
		t.Error("FailOnSastScan = true, want false")
	}
	if config.MinimumLicenseSeverity != "critical" {
		t.Errorf("MinimumLicenseSeverity = %q, want %q", config.MinimumLicenseSeverity, "critical")
	}
	if config.PostInlineCommentsMinSeverity != nil {
		t.Errorf("PostInlineCommentsMinSeverity = %q, want nil", *config.PostInlineCommentsMinSeverity)
	}
	if config.PostCodeQualityInlineCommentsMinSeverity != "none" {
		t.Errorf("PostCodeQualityInlineCommentsMinSeverity = %q, want %q",
			config.PostCodeQualityInlineCommentsMinSeverity, "none")
	}
	if !config.RunDeepAuditPRScan {
		t.Error("RunDeepAuditPRScan = false, want true")
	}
}

func TestGetCIChecksDefaultConfiguration_Error(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		mustEncode(t, w, map[string]string{"error": "missing repositories:read scope"})
	})
	defer server.Close()

	_, err := c.GetCIChecksDefaultConfiguration(context.Background())
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := err.Error(); !strings.Contains(got, "missing repositories:read scope") {
		t.Errorf("error = %q, want it to include the API message", got)
	}
}

func TestSaveCIChecksDefaultConfiguration(t *testing.T) {
	codeQualitySeverity := "medium"

	var body map[string]interface{}
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/public/v1/repositories/code/continuous_integration/checks/default" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		mustDecode(t, r, &body)
		mustEncode(t, w, map[string]string{"status": "ok"})
	})
	defer server.Close()

	err := c.SaveCIChecksDefaultConfiguration(context.Background(), SaveCIChecksDefaultConfigurationRequest{
		MinimumSeverity:                          "high",
		FailOnDependencyScan:                     true,
		MinimumLicenseSeverity:                   "none",
		PostInlineCommentsMinSeverity:            "critical",
		EnableCodeQualityScan:                    true,
		PostCodeQualityInlineCommentsMinSeverity: &codeQualitySeverity,
	})
	if err != nil {
		t.Fatalf("SaveCIChecksDefaultConfiguration() error = %v", err)
	}

	if got := body["minimum_severity"]; got != "high" {
		t.Errorf("minimum_severity = %v, want high", got)
	}
	if got := body["fail_on_dependency_scan"]; got != true {
		t.Errorf("fail_on_dependency_scan = %v, want true", got)
	}
	if got := body["post_inline_comments_min_severity"]; got != "critical" {
		t.Errorf("post_inline_comments_min_severity = %v, want critical", got)
	}
	if got := body["post_code_quality_inline_comments_min_severity"]; got != "medium" {
		t.Errorf("post_code_quality_inline_comments_min_severity = %v, want medium", got)
	}
	// The scan flags must be sent even when false: all of them false is how the default is removed.
	if got, present := body["run_deep_audit_pr_scan"]; !present || got != false {
		t.Errorf("run_deep_audit_pr_scan = %v (present %v), want an explicit false", got, present)
	}
}

func TestSaveCIChecksDefaultConfiguration_CodeQualitySeverityOmitted(t *testing.T) {
	var body map[string]interface{}
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		mustDecode(t, r, &body)
		mustEncode(t, w, map[string]string{"status": "ok"})
	})
	defer server.Close()

	err := c.SaveCIChecksDefaultConfiguration(context.Background(), SaveCIChecksDefaultConfigurationRequest{
		MinimumSeverity:               "low",
		MinimumLicenseSeverity:        "none",
		PostInlineCommentsMinSeverity: "none",
	})
	if err != nil {
		t.Fatalf("SaveCIChecksDefaultConfiguration() error = %v", err)
	}

	if _, present := body["post_code_quality_inline_comments_min_severity"]; present {
		t.Errorf("post_code_quality_inline_comments_min_severity must be omitted when nil, got %v",
			body["post_code_quality_inline_comments_min_severity"])
	}
}

func TestSaveCIChecksDefaultConfiguration_Error(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		mustEncode(t, w, map[string]string{"error": "Deep Review is not available in this region"})
	})
	defer server.Close()

	err := c.SaveCIChecksDefaultConfiguration(context.Background(), SaveCIChecksDefaultConfigurationRequest{})
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := err.Error(); !strings.Contains(got, "Deep Review is not available in this region") {
		t.Errorf("error = %q, want it to include the API message", got)
	}
}

func TestSaveCIChecksConfiguration_Error(t *testing.T) {
	server, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		mustEncode(t, w, map[string]string{"error": "invalid minimum_severity"})
	})
	defer server.Close()

	err := c.SaveCIChecksConfiguration(context.Background(), SaveCIChecksConfigurationRequest{CodeRepoID: 42})
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := err.Error(); !strings.Contains(got, "invalid minimum_severity") {
		t.Errorf("error = %q, want it to include the API message", got)
	}
}
