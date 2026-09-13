package provider_test

import (
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"testing"
)

func TestAccMaintenanceWindowAndScheduleOverride(t *testing.T) {
	testAccPreCheck(t)
	config := providerConfig() + `
resource "anomaly_user" "release" { name = "Release test user" }
resource "anomaly_schedule" "release" { name = "Release test schedule" }
resource "anomaly_integration" "release" { name = "Release test integration" }
resource "anomaly_schedule_override" "release" {
  schedule_id = anomaly_schedule.release.id
  user_id = anomaly_user.release.id
  start_at = "2027-01-04T09:00:00Z"
  until = "2027-01-05T09:00:00Z"
  reason = "Release test"
}
resource "anomaly_maintenance_window" "release" {
  name = "Release test maintenance"
  integration_ids = [anomaly_integration.release.id]
  starts_at = "2027-01-04T09:00:00+00:00"
  ends_at = "2027-01-05T09:00:00+00:00"
  reason = "Release test"
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config, Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttrSet("anomaly_schedule_override.release", "id"),
				resource.TestCheckResourceAttrSet("anomaly_maintenance_window.release", "id"),
			)},
			{ResourceName: "anomaly_schedule_override.release", ImportState: true, ImportStateVerify: true,
				ImportStateIdFunc: func(state *terraform.State) (string, error) {
					attrs := state.RootModule().Resources["anomaly_schedule_override.release"].Primary.Attributes
					return attrs["schedule_id"] + "/" + attrs["id"], nil
				},
			},
			{ResourceName: "anomaly_maintenance_window.release", ImportState: true, ImportStateVerify: true},
		},
	})
}
