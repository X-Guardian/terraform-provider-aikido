package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccTeamResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: testAccTeamResourceConfig("tf-acc-test-team"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("aikido_team.test", tfjsonpath.New("name"), knownvalue.StringExact("tf-acc-test-team")),
					statecheck.ExpectKnownValue("aikido_team.test", tfjsonpath.New("id"), knownvalue.NotNull()),
				},
			},
			// ImportState
			{
				ResourceName:      "aikido_team.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update name
			{
				Config: testAccTeamResourceConfig("tf-acc-test-team-updated"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("aikido_team.test", tfjsonpath.New("name"), knownvalue.StringExact("tf-acc-test-team-updated")),
				},
			},
			// Delete is implicit at the end of the test
		},
	})
}

func testAccTeamResourceConfig(name string) string {
	return fmt.Sprintf(`
resource "aikido_team" "test" {
  name = %[1]q
}
`, name)
}
