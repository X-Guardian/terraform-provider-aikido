package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccAutofixSastResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: testAccAutofixSastResourceConfig,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("aikido_autofix_sast.test", tfjsonpath.New("id"), knownvalue.StringExact("sast")),
					statecheck.ExpectKnownValue("aikido_autofix_sast.test", tfjsonpath.New("enabled"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("aikido_autofix_sast.test", tfjsonpath.New("severity_filter"), knownvalue.StringExact("critical_and_high_only")),
					statecheck.ExpectKnownValue("aikido_autofix_sast.test", tfjsonpath.New("repos_scope"), knownvalue.StringExact("all")),
					// Scope "all" stores an empty set so state matches what the API keeps.
					statecheck.ExpectKnownValue("aikido_autofix_sast.test", tfjsonpath.New("repo_ids"), knownvalue.SetSizeExact(0)),
				},
			},
			// ImportState. The settings are a workspace-wide singleton, so the import ID is
			// ignored and normalised to the sentinel.
			{
				ResourceName:      "aikido_autofix_sast.test",
				ImportState:       true,
				ImportStateId:     "sast",
				ImportStateVerify: true,
			},
			// Update the severity filter in place
			{
				Config: testAccAutofixSastResourceConfigUpdated,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("aikido_autofix_sast.test", tfjsonpath.New("severity_filter"), knownvalue.StringExact("all")),
				},
			},
			// Disabling only requires enabled. The subordinate values stay as configured
			// rather than being refreshed from the API, which reports them as "none".
			{
				Config: testAccAutofixSastResourceConfigDisabled,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("aikido_autofix_sast.test", tfjsonpath.New("enabled"), knownvalue.Bool(false)),
					statecheck.ExpectKnownValue("aikido_autofix_sast.test", tfjsonpath.New("severity_filter"), knownvalue.Null()),
				},
			},
			// Delete is implicit at the end of the test and disables autofix
		},
	})
}

const testAccAutofixSastResourceConfig = `
resource "aikido_autofix_sast" "test" {
  enabled         = true
  severity_filter = "critical_and_high_only"
  repos_scope     = "all"
}
`

const testAccAutofixSastResourceConfigUpdated = `
resource "aikido_autofix_sast" "test" {
  enabled         = true
  severity_filter = "all"
  repos_scope     = "all"
}
`

const testAccAutofixSastResourceConfigDisabled = `
resource "aikido_autofix_sast" "test" {
  enabled = false
}
`
