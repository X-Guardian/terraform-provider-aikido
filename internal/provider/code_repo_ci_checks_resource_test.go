package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/X-Guardian/terraform-provider-aikido/internal/client"
)

func TestMapCIChecksToModel(t *testing.T) {
	severity := "high"
	config := &client.CIChecksConfiguration{
		ID:                                       237367,
		CodeRepoID:                               410167,
		MinimumSeverity:                          "high",
		FailOnDependencyScan:                     true,
		FailOnSastScan:                           true,
		FailOnIacScan:                            false,
		FailOnSecretsScan:                        true,
		FailOnMalwareScan:                        false,
		PostInlineCommentsMinSeverity:            "critical",
		MinimumLicenseSeverity:                   "critical",
		EnableCodeQualityScan:                    true,
		FailOnCodeQualityScan:                    false,
		PostCodeQualityInlineCommentsMinSeverity: &severity,
		RunDeepAuditPRScan:                       true,
	}

	var data CodeRepoCIChecksResourceModel
	mapCIChecksToModel(config, &data)

	// The resource ID is the code repo ID, not the configuration's own ID.
	if got := data.ID.ValueString(); got != "410167" {
		t.Errorf("ID = %q, want %q", got, "410167")
	}
	if got := data.CodeRepoID.ValueString(); got != "410167" {
		t.Errorf("CodeRepoID = %q, want %q", got, "410167")
	}
	if got := data.MinimumSeverity.ValueString(); got != "high" {
		t.Errorf("MinimumSeverity = %q, want %q", got, "high")
	}
	if !data.FailOnDependencyScan.ValueBool() || !data.FailOnSastScan.ValueBool() || !data.FailOnSecretsScan.ValueBool() {
		t.Error("expected dependency, sast and secrets failures to be true")
	}
	if data.FailOnIacScan.ValueBool() || data.FailOnMalwareScan.ValueBool() {
		t.Error("expected iac and malware failures to be false")
	}
	if got := data.MinimumLicenseSeverity.ValueString(); got != "critical" {
		t.Errorf("MinimumLicenseSeverity = %q, want %q", got, "critical")
	}
	if !data.EnableCodeQualityScan.ValueBool() {
		t.Error("EnableCodeQualityScan = false, want true")
	}
	if got := data.PostCodeQualityInlineCommentsMinSeverity.ValueString(); got != "high" {
		t.Errorf("PostCodeQualityInlineCommentsMinSeverity = %q, want %q", got, "high")
	}
	if got := data.PostInlineCommentsMinSeverity.ValueString(); got != "critical" {
		t.Errorf("PostInlineCommentsMinSeverity = %q, want %q", got, "critical")
	}
	if !data.RunDeepAuditPRScan.ValueBool() {
		t.Error("RunDeepAuditPRScan = false, want true")
	}
}

func TestMapCIChecksToModel_NullCodeQualitySeverity(t *testing.T) {
	config := &client.CIChecksConfiguration{
		CodeRepoID:                               42,
		EnableCodeQualityScan:                    false,
		PostCodeQualityInlineCommentsMinSeverity: nil,
	}

	var data CodeRepoCIChecksResourceModel
	mapCIChecksToModel(config, &data)

	if !data.PostCodeQualityInlineCommentsMinSeverity.IsNull() {
		t.Errorf("PostCodeQualityInlineCommentsMinSeverity = %q, want null",
			data.PostCodeQualityInlineCommentsMinSeverity.ValueString())
	}
}

// TestMapCIChecksToModel_PreservesDeepAuditSeverity pins the write-only handling: the API does not return
// post_deep_audit_inline_comments_min_severity, so a read must leave the prior value alone rather than reporting a
// change to null.
func TestMapCIChecksToModel_PreservesDeepAuditSeverity(t *testing.T) {
	data := CodeRepoCIChecksResourceModel{
		PostDeepAuditInlineCommentsMinSeverity: types.StringValue("medium"),
	}

	mapCIChecksToModel(&client.CIChecksConfiguration{CodeRepoID: 42}, &data)

	if got := data.PostDeepAuditInlineCommentsMinSeverity.ValueString(); got != "medium" {
		t.Errorf("PostDeepAuditInlineCommentsMinSeverity = %q, want it preserved as %q", got, "medium")
	}
}

// TestCIChecksCodeQualityValidator covers the documented code quality rule: the comment severity is only meaningful
// when the scan is enabled. Requiring it when the scan is enabled is deliberately not enforced, because the API
// documents no such requirement.
func TestCIChecksCodeQualityValidator(t *testing.T) {
	tests := []struct {
		name        string
		enabled     types.Bool
		severity    types.String
		expectError bool
	}{
		{
			name:        "enabled with a severity is valid",
			enabled:     types.BoolValue(true),
			severity:    types.StringValue("high"),
			expectError: false,
		},
		{
			// The API documents no requirement to supply a severity when the scan is on, and a live apply confirmed it
			// accepts a null severity in that case. Enforcing one here would reject configurations the API accepts.
			name:        "enabled without a severity is allowed",
			enabled:     types.BoolValue(true),
			severity:    types.StringNull(),
			expectError: false,
		},
		{
			name:        "disabled without a severity is valid",
			enabled:     types.BoolValue(false),
			severity:    types.StringNull(),
			expectError: false,
		},
		{
			name:        "disabled with a severity is rejected",
			enabled:     types.BoolValue(false),
			severity:    types.StringValue("high"),
			expectError: true,
		},
		{
			name:        "an unknown scan flag is not validated",
			enabled:     types.BoolUnknown(),
			severity:    types.StringNull(),
			expectError: false,
		},
		{
			name:        "an unknown severity is not validated",
			enabled:     types.BoolValue(true),
			severity:    types.StringUnknown(),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := validateCIChecksCodeQuality(tt.enabled, tt.severity)
			if got := diags.HasError(); got != tt.expectError {
				t.Errorf("HasError() = %v, want %v (diags: %v)", got, tt.expectError, diags)
			}
		})
	}
}

func TestIsKnown(t *testing.T) {
	tests := []struct {
		name     string
		value    types.String
		expected bool
	}{
		{name: "a set value is known", value: types.StringValue("high"), expected: true},
		{name: "an empty string is still known", value: types.StringValue(""), expected: true},
		{name: "null is not known", value: types.StringNull(), expected: false},
		{name: "unknown is not known", value: types.StringUnknown(), expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isKnown(tt.value); got != tt.expected {
				t.Errorf("isKnown() = %v, want %v", got, tt.expected)
			}
		})
	}
}
