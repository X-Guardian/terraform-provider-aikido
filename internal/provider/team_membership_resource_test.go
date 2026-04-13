package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccTeamMembershipResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: testAccTeamMembershipResourceConfig("tf-acc-membership-team", "1"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("aikido_team_membership.test", tfjsonpath.New("team_id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("aikido_team_membership.test", tfjsonpath.New("user_id"), knownvalue.StringExact("1")),
				},
			},
			// ImportState
			{
				ResourceName:      "aikido_team_membership.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Delete is implicit at the end of the test
		},
	})
}

func testAccTeamMembershipResourceConfig(teamName, userID string) string {
	return fmt.Sprintf(`
resource "aikido_team" "test" {
  name = %[1]q
}

resource "aikido_team_membership" "test" {
  team_id = aikido_team.test.id
  user_id = %[2]q
}
`, teamName, userID)
}
