package provider

import (
	"testing"

	"github.com/X-Guardian/terraform-provider-aikido/internal/client"
)

// Every computed attribute in the schema must be assigned by mapAppToModel.
// An unassigned one stays null here but reaches Terraform as unknown after
// apply, which it rejects with "Provider returned invalid result object after
// apply" and taints the resource — what shipped in 0.8.1 with blocking unset.
//
// The null/unknown split is why the checks below assert on values and not
// just IsUnknown(): a missing assignment leaves a zero-valued attribute that
// IsUnknown() reports as known.
func TestMapAppToModel_SetsAllComputedAttributes(t *testing.T) {
	repoName := "my-repo"
	tests := []struct {
		name string
		app  client.ZenAppDetail
	}{
		{
			name: "linked repo",
			app: client.ZenAppDetail{
				ID:           42,
				Name:         "my-app",
				Environment:  "production",
				HasToken:     true,
				TokenHint:    "IFgj",
				Blocking:     true,
				CodeRepoID:   7,
				CodeRepoName: &repoName,
			},
		},
		{
			// -1 is what the API sends for an unlinked app.
			name: "unlinked repo",
			app: client.ZenAppDetail{
				ID:          74258,
				Name:        "portal (staging, uk)",
				Environment: "staging",
				HasToken:    true,
				TokenHint:   "IFgj",
				Blocking:    false,
				CodeRepoID:  -1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ZenAppResource{}
			var data ZenAppResourceModel
			r.mapAppToModel(&tt.app, &data)

			// Token is deliberately excluded: it is only returned at create
			// time and mapAppToModel must not overwrite it. code_repo_name is
			// legitimately null for an unlinked app, so it is asserted per
			// case below rather than here.
			unset := map[string]bool{
				"id":         data.ID.IsNull() || data.ID.IsUnknown(),
				"token_hint": data.TokenHint.IsNull() || data.TokenHint.IsUnknown(),
				"has_token":  data.HasToken.IsNull() || data.HasToken.IsUnknown(),
				"blocking":   data.Blocking.IsNull() || data.Blocking.IsUnknown(),
			}
			for attr, isUnset := range unset {
				if isUnset {
					t.Errorf("computed attribute %q was never assigned", attr)
				}
			}

			if data.Blocking.ValueBool() != tt.app.Blocking {
				t.Errorf("expected blocking %v, got %v", tt.app.Blocking, data.Blocking.ValueBool())
			}
		})
	}
}
