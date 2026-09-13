package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTeamResource_lifecycle(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create without members
			{
				Config: providerConfig() + `
resource "anomaly_team" "test" {
  name = "TF Test Team"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("anomaly_team.test", "id"),
					resource.TestCheckResourceAttr("anomaly_team.test", "name", "TF Test Team"),
				),
			},
			// Update name
			{
				Config: providerConfig() + `
resource "anomaly_team" "test" {
  name = "TF Test Team Renamed"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("anomaly_team.test", "name", "TF Test Team Renamed"),
				),
			},
		},
	})
}

func TestAccTeamResource_withMembers(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "anomaly_user" "member" {
  name = "Team Member TF"
}

resource "anomaly_team" "with_members" {
  name       = "Team With Members"
  member_ids = [anomaly_user.member.id]
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("anomaly_team.with_members", "id"),
					resource.TestCheckResourceAttr("anomaly_team.with_members", "member_ids.#", "1"),
				),
			},
		},
	})
}
