package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/X-Guardian/terraform-provider-aikido/internal/client"
)

func TestMapCIChecksDefaultToModel(t *testing.T) {
	inlineSeverity := "critical"
	config := &client.CIChecksDefaultConfiguration{
		IsEnabled:                                true,
		MinimumSeverity:                          "high",
		FailOnDependencyScan:                     true,
		FailOnSastScan:                           true,
		FailOnIacScan:                            false,
		FailOnSecretsScan:                        true,
		FailOnMalwareScan:                        false,
		MinimumLicenseSeverity:                   "critical",
		PostInlineComments:                       true,
		PostInlineCommentsMinSeverity:            &inlineSeverity,
		EnableCodeQualityScan:                    true,
		PostCodeQualityInlineCommentsMinSeverity: "medium",
		FailOnCodeQualityScan:                    true,
		RunDeepAuditPRScan:                       true,
	}

	var data CodeRepoCIChecksDefaultResourceModel
	mapCIChecksDefaultToModel(config, &data)

	if got := data.ID.ValueString(); got != ciChecksDefaultID {
		t.Errorf("ID = %q, want %q", got, ciChecksDefaultID)
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
	if got := data.PostInlineCommentsMinSeverity.ValueString(); got != "critical" {
		t.Errorf("PostInlineCommentsMinSeverity = %q, want %q", got, "critical")
	}
	if !data.EnableCodeQualityScan.ValueBool() || !data.FailOnCodeQualityScan.ValueBool() {
		t.Error("expected the code quality scan to be enabled and failing")
	}
	if got := data.PostCodeQualityInlineCommentsMinSeverity.ValueString(); got != "medium" {
		t.Errorf("PostCodeQualityInlineCommentsMinSeverity = %q, want %q", got, "medium")
	}
	if !data.RunDeepAuditPRScan.ValueBool() {
		t.Error("RunDeepAuditPRScan = false, want true")
	}
}

// TestMapCIChecksDefaultToModel_DisabledComments pins the translation of the API's read representation of "off": a
// null inline comment severity reads as the "none" the configuration uses, and the "none" code quality severity reads
// as null, which is what the plan modifier plans when the scan is disabled.
func TestMapCIChecksDefaultToModel_DisabledComments(t *testing.T) {
	config := &client.CIChecksDefaultConfiguration{
		IsEnabled:                                true,
		PostInlineCommentsMinSeverity:            nil,
		EnableCodeQualityScan:                    false,
		PostCodeQualityInlineCommentsMinSeverity: "none",
	}

	var data CodeRepoCIChecksDefaultResourceModel
	mapCIChecksDefaultToModel(config, &data)

	if got := data.PostInlineCommentsMinSeverity.ValueString(); got != "none" {
		t.Errorf("PostInlineCommentsMinSeverity = %q, want %q", got, "none")
	}
	if !data.PostCodeQualityInlineCommentsMinSeverity.IsNull() {
		t.Errorf("PostCodeQualityInlineCommentsMinSeverity = %q, want null",
			data.PostCodeQualityInlineCommentsMinSeverity.ValueString())
	}
}

func TestCIChecksDefaultCodeQualitySeverityFor(t *testing.T) {
	tests := []struct {
		name     string
		enabled  types.Bool
		expected types.String
	}{
		{name: "enabled plans the API default", enabled: types.BoolValue(true), expected: types.StringValue("low")},
		{name: "disabled plans null", enabled: types.BoolValue(false), expected: types.StringNull()},
		{name: "unknown stays unknown", enabled: types.BoolUnknown(), expected: types.StringUnknown()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ciChecksDefaultCodeQualitySeverityFor(tt.enabled); !got.Equal(tt.expected) {
				t.Errorf("ciChecksDefaultCodeQualitySeverityFor() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestValidateCIChecksDefault(t *testing.T) {
	// allOff returns a configuration with every scan flag false, for the cases to switch individual flags back on.
	allOff := func() CodeRepoCIChecksDefaultResourceModel {
		return CodeRepoCIChecksDefaultResourceModel{
			FailOnDependencyScan:  types.BoolValue(false),
			FailOnSastScan:        types.BoolValue(false),
			FailOnIacScan:         types.BoolValue(false),
			FailOnSecretsScan:     types.BoolValue(false),
			FailOnMalwareScan:     types.BoolValue(false),
			EnableCodeQualityScan: types.BoolValue(false),
			FailOnCodeQualityScan: types.BoolValue(false),
			RunDeepAuditPRScan:    types.BoolNull(),
		}
	}

	tests := []struct {
		name        string
		modify      func(*CodeRepoCIChecksDefaultResourceModel)
		expectError bool
	}{
		{
			name:        "every scan off is rejected",
			modify:      func(*CodeRepoCIChecksDefaultResourceModel) {},
			expectError: true,
		},
		{
			name:        "one vulnerability scan on is valid",
			modify:      func(d *CodeRepoCIChecksDefaultResourceModel) { d.FailOnSecretsScan = types.BoolValue(true) },
			expectError: false,
		},
		{
			name:        "only the code quality scan on is valid",
			modify:      func(d *CodeRepoCIChecksDefaultResourceModel) { d.EnableCodeQualityScan = types.BoolValue(true) },
			expectError: false,
		},
		{
			name:        "only Deep Review on is valid",
			modify:      func(d *CodeRepoCIChecksDefaultResourceModel) { d.RunDeepAuditPRScan = types.BoolValue(true) },
			expectError: false,
		},
		{
			name:        "an unknown scan flag is not validated",
			modify:      func(d *CodeRepoCIChecksDefaultResourceModel) { d.FailOnSastScan = types.BoolUnknown() },
			expectError: false,
		},
		{
			name: "failing on code quality with the scan off is rejected",
			modify: func(d *CodeRepoCIChecksDefaultResourceModel) {
				d.FailOnDependencyScan = types.BoolValue(true)
				d.FailOnCodeQualityScan = types.BoolValue(true)
			},
			expectError: true,
		},
		{
			name: "failing on code quality with the scan on is valid",
			modify: func(d *CodeRepoCIChecksDefaultResourceModel) {
				d.EnableCodeQualityScan = types.BoolValue(true)
				d.FailOnCodeQualityScan = types.BoolValue(true)
			},
			expectError: false,
		},
		{
			name: "failing on code quality with an unknown scan flag is not validated",
			modify: func(d *CodeRepoCIChecksDefaultResourceModel) {
				d.EnableCodeQualityScan = types.BoolUnknown()
				d.FailOnCodeQualityScan = types.BoolValue(true)
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := allOff()
			tt.modify(&data)
			diags := validateCIChecksDefault(&data)
			if got := diags.HasError(); got != tt.expectError {
				t.Errorf("HasError() = %v, want %v (diags: %v)", got, tt.expectError, diags)
			}
		})
	}
}
