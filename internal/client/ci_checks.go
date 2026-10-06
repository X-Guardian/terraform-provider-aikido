package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

// ciChecksPath is the CI checks (PR gating) configuration endpoint. A single path serves both the paginated list and
// the create-or-update upsert.
const ciChecksPath = "/repositories/code/continuous_integration/checks"

// ciChecksDefaultPath is the workspace default CI checks configuration endpoint, applied to newly activated
// repositories that have no repository-specific configuration.
const ciChecksDefaultPath = ciChecksPath + "/default"

// ciChecksPageSize is the API maximum documented for per_page on the endpoint.
const ciChecksPageSize = 100

// ErrCIChecksNotFound is returned when a code repository has no CI checks configuration. Unlike the workspace-wide
// autofix settings, a per-repository configuration may genuinely not exist yet, so callers treat this as "not created"
// rather than a failure.
var ErrCIChecksNotFound = errors.New("ci checks configuration not found")

// CIChecksConfiguration is a code repository's CI checks configuration.
//
// PostCodeQualityInlineCommentsMinSeverity is a pointer because the API returns null for it when the code quality scan
// is disabled.
//
// post_deep_audit_inline_comments_min_severity is deliberately absent: the API accepts it on write but does not return
// it, so there is nothing to decode.
type CIChecksConfiguration struct {
	ID                                       int     `json:"id"`
	CodeRepoID                               int     `json:"code_repo_id"`
	MinimumSeverity                          string  `json:"minimum_severity"`
	FailOnDependencyScan                     bool    `json:"fail_on_dependency_scan"`
	FailOnSastScan                           bool    `json:"fail_on_sast_scan"`
	FailOnIacScan                            bool    `json:"fail_on_iac_scan"`
	FailOnSecretsScan                        bool    `json:"fail_on_secrets_scan"`
	FailOnMalwareScan                        bool    `json:"fail_on_malware_scan"`
	PostInlineCommentsMinSeverity            string  `json:"post_inline_comments_min_severity"`
	MinimumLicenseSeverity                   string  `json:"minimum_license_severity"`
	EnableCodeQualityScan                    bool    `json:"enable_code_quality_scan"`
	FailOnCodeQualityScan                    bool    `json:"fail_on_code_quality_scan"`
	PostCodeQualityInlineCommentsMinSeverity *string `json:"post_code_quality_inline_comments_min_severity"`
	RunDeepAuditPRScan                       bool    `json:"run_deep_audit_pr_scan"`
}

// SaveCIChecksConfigurationRequest is the POST body for the CI checks configuration endpoint, which upserts: the same
// call creates a configuration for a repository that has none and replaces the configuration of one that does.
//
// Every field the API marks required must be sent on every call, so those are plain values: a partial update is not
// possible.
//
// PostCodeQualityInlineCommentsMinSeverity must be present but may be null, so it is a pointer with a plain json tag
// rather than omitempty: a nil pointer marshals to an explicit null instead of dropping the key.
//
// PostDeepAuditInlineCommentsMinSeverity is accepted here but never returned by the GET, so it can be written and not
// read back. The provider cannot detect drift on it.
type SaveCIChecksConfigurationRequest struct {
	CodeRepoID                               int     `json:"code_repo_id"`
	MinimumSeverity                          string  `json:"minimum_severity"`
	FailOnDependencyScan                     bool    `json:"fail_on_dependency_scan"`
	FailOnSastScan                           bool    `json:"fail_on_sast_scan"`
	FailOnIacScan                            bool    `json:"fail_on_iac_scan"`
	FailOnSecretsScan                        bool    `json:"fail_on_secrets_scan"`
	FailOnMalwareScan                        bool    `json:"fail_on_malware_scan"`
	MinimumLicenseSeverity                   string  `json:"minimum_license_severity"`
	EnableCodeQualityScan                    bool    `json:"enable_code_quality_scan"`
	FailOnCodeQualityScan                    bool    `json:"fail_on_code_quality_scan"`
	PostCodeQualityInlineCommentsMinSeverity *string `json:"post_code_quality_inline_comments_min_severity"`
	PostInlineCommentsMinSeverity            *string `json:"post_inline_comments_min_severity,omitempty"`
	RunDeepAuditPRScan                       *bool   `json:"run_deep_audit_pr_scan,omitempty"`
	PostDeepAuditInlineCommentsMinSeverity   *string `json:"post_deep_audit_inline_comments_min_severity,omitempty"`
}

// GetCIChecksConfiguration retrieves the CI checks configuration for a single code repository, using
// filter_code_repo_id so the lookup costs one request rather than a full paginated scan.
//
// The returned error wraps ErrCIChecksNotFound when the repository has no configuration yet, which is a normal state
// rather than a failure.
//
// The ID is still matched client-side and pagination still advances on a full page, so a server that ignores the filter
// yields the same result rather than the wrong one.
func (c *AikidoClient) GetCIChecksConfiguration(ctx context.Context, repoID int) (*CIChecksConfiguration, error) {
	page := 0
	for {
		params := url.Values{}
		params.Set("page", strconv.Itoa(page))
		params.Set("per_page", strconv.Itoa(ciChecksPageSize))
		params.Set("filter_code_repo_id", strconv.Itoa(repoID))

		configs, err := c.getCIChecksPage(ctx, params)
		if err != nil {
			return nil, err
		}

		for i := range configs {
			if configs[i].CodeRepoID == repoID {
				return &configs[i], nil
			}
		}

		if len(configs) < ciChecksPageSize {
			return nil, fmt.Errorf("ci checks configuration for code repo %d: %w", repoID, ErrCIChecksNotFound)
		}

		page++
	}
}

// SaveCIChecksConfiguration creates or replaces a code repository's CI checks configuration. The endpoint upserts, so
// there is no separate create and update, and the {"success": 1} response body carries nothing useful.
func (c *AikidoClient) SaveCIChecksConfiguration(ctx context.Context, req SaveCIChecksConfigurationRequest) error {
	resp, err := c.DoRequest(ctx, http.MethodPost, ciChecksPath, req)
	if err != nil {
		return fmt.Errorf("saving ci checks configuration: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d saving ci checks configuration: %s", resp.StatusCode, errorBody(body))
	}

	return nil
}

// CIChecksDefaultConfiguration is the workspace default CI checks configuration.
//
// When no default exists the API returns fallback values with IsEnabled false rather than a 404, so IsEnabled is what
// distinguishes a configured default from an absent one.
//
// PostInlineCommentsMinSeverity is a pointer because the API returns null for it when inline comments are disabled.
// PostCodeQualityInlineCommentsMinSeverity is "none" rather than null when the code quality scan is disabled.
// FailOnLicenseScan and PostInlineComments are derived by the API from the severities and are not settable.
type CIChecksDefaultConfiguration struct {
	IsEnabled                                bool    `json:"is_enabled"`
	MinimumSeverity                          string  `json:"minimum_severity"`
	FailOnDependencyScan                     bool    `json:"fail_on_dependency_scan"`
	FailOnSastScan                           bool    `json:"fail_on_sast_scan"`
	FailOnIacScan                            bool    `json:"fail_on_iac_scan"`
	FailOnSecretsScan                        bool    `json:"fail_on_secrets_scan"`
	FailOnMalwareScan                        bool    `json:"fail_on_malware_scan"`
	FailOnLicenseScan                        bool    `json:"fail_on_license_scan"`
	MinimumLicenseSeverity                   string  `json:"minimum_license_severity"`
	PostInlineComments                       bool    `json:"post_inline_comments"`
	PostInlineCommentsMinSeverity            *string `json:"post_inline_comments_min_severity"`
	EnableCodeQualityScan                    bool    `json:"enable_code_quality_scan"`
	PostCodeQualityInlineCommentsMinSeverity string  `json:"post_code_quality_inline_comments_min_severity"`
	FailOnCodeQualityScan                    bool    `json:"fail_on_code_quality_scan"`
	RunDeepAuditPRScan                       bool    `json:"run_deep_audit_pr_scan"`
}

// SaveCIChecksDefaultConfigurationRequest is the POST body for the default CI checks configuration endpoint, which
// creates or replaces the workspace default.
//
// The API removes the default when every scan flag (the five fail_on_* vulnerability flags, enable_code_quality_scan
// and run_deep_audit_pr_scan) is false, so that is also how it is deleted.
//
// PostCodeQualityInlineCommentsMinSeverity is omitted when nil: the API ignores it when the code quality scan is
// disabled, and its enum has no null.
type SaveCIChecksDefaultConfigurationRequest struct {
	MinimumSeverity                          string  `json:"minimum_severity"`
	FailOnDependencyScan                     bool    `json:"fail_on_dependency_scan"`
	FailOnSastScan                           bool    `json:"fail_on_sast_scan"`
	FailOnIacScan                            bool    `json:"fail_on_iac_scan"`
	FailOnSecretsScan                        bool    `json:"fail_on_secrets_scan"`
	FailOnMalwareScan                        bool    `json:"fail_on_malware_scan"`
	MinimumLicenseSeverity                   string  `json:"minimum_license_severity"`
	PostInlineCommentsMinSeverity            string  `json:"post_inline_comments_min_severity"`
	EnableCodeQualityScan                    bool    `json:"enable_code_quality_scan"`
	FailOnCodeQualityScan                    bool    `json:"fail_on_code_quality_scan"`
	PostCodeQualityInlineCommentsMinSeverity *string `json:"post_code_quality_inline_comments_min_severity,omitempty"`
	RunDeepAuditPRScan                       bool    `json:"run_deep_audit_pr_scan"`
}

// GetCIChecksDefaultConfiguration retrieves the workspace default CI checks configuration.
func (c *AikidoClient) GetCIChecksDefaultConfiguration(ctx context.Context) (*CIChecksDefaultConfiguration, error) {
	resp, err := c.DoRequest(ctx, http.MethodGet, ciChecksDefaultPath, nil)
	if err != nil {
		return nil, fmt.Errorf("getting default ci checks configuration: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d getting default ci checks configuration: %s", resp.StatusCode, errorBody(body))
	}

	var config CIChecksDefaultConfiguration
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return nil, fmt.Errorf("decoding default ci checks configuration response: %w", err)
	}

	return &config, nil
}

// SaveCIChecksDefaultConfiguration creates, replaces or (with every scan flag false) removes the workspace default CI
// checks configuration. The {"status": "ok"} response body carries nothing useful.
func (c *AikidoClient) SaveCIChecksDefaultConfiguration(ctx context.Context, req SaveCIChecksDefaultConfigurationRequest) error {
	resp, err := c.DoRequest(ctx, http.MethodPost, ciChecksDefaultPath, req)
	if err != nil {
		return fmt.Errorf("saving default ci checks configuration: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d saving default ci checks configuration: %s", resp.StatusCode, errorBody(body))
	}

	return nil
}

// getCIChecksPage fetches a single page of CI checks configurations.
func (c *AikidoClient) getCIChecksPage(ctx context.Context, params url.Values) ([]CIChecksConfiguration, error) {
	resp, err := c.DoRequest(ctx, http.MethodGet, ciChecksPath+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("listing ci checks configurations: %w", err)
	}
	defer resp.Body.Close()

	// A repository with no configuration is a normal state, so a 404 is mapped to the sentinel rather than a failure.
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("listing ci checks configurations: %w", ErrCIChecksNotFound)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d listing ci checks configurations: %s", resp.StatusCode, errorBody(body))
	}

	// The API spec declares this response as the single-item object, but the endpoint returns an array, matching the
	// identically-declared GET /repositories/code.
	var configs []CIChecksConfiguration
	if err := json.NewDecoder(resp.Body).Decode(&configs); err != nil {
		return nil, fmt.Errorf("decoding ci checks configurations response: %w", err)
	}

	return configs, nil
}
