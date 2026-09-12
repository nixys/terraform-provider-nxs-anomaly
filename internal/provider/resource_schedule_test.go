package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccScheduleResource_lifecycle(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create schedule with one shift
			{
				Config: providerConfig() + `
resource "anomaly_user" "on_call" {
  name = "On-Call User TF"
}

resource "anomaly_schedule" "test" {
  name     = "TF Test Schedule"
  timezone = "UTC"
  shifts = [
    {
      user_id    = anomaly_user.on_call.id
      start_at   = "2026-07-01T09:00:00Z"
      end_at     = "2026-07-01T17:00:00Z"
      recurrence = "weekly"
    }
  ]
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("anomaly_schedule.test", "id"),
					resource.TestCheckResourceAttr("anomaly_schedule.test", "name", "TF Test Schedule"),
					resource.TestCheckResourceAttr("anomaly_schedule.test", "timezone", "UTC"),
					resource.TestCheckResourceAttr("anomaly_schedule.test", "shifts.#", "1"),
					resource.TestCheckResourceAttr("anomaly_schedule.test", "shifts.0.recurrence", "weekly"),
				),
			},
			// Update: rename + clear shifts
			{
				Config: providerConfig() + `
resource "anomaly_user" "on_call" {
  name = "On-Call User TF"
}

resource "anomaly_schedule" "test" {
  name     = "TF Test Schedule Renamed"
  timezone = "Europe/Moscow"
  shifts   = []
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("anomaly_schedule.test", "name", "TF Test Schedule Renamed"),
					resource.TestCheckResourceAttr("anomaly_schedule.test", "timezone", "Europe/Moscow"),
					resource.TestCheckResourceAttr("anomaly_schedule.test", "shifts.#", "0"),
				),
			},
		},
	})
}

func TestAccScheduleResource_withTeam(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "anomaly_team" "sched_team" {
  name = "Schedule Team TF"
}

resource "anomaly_schedule" "with_team" {
  name    = "Schedule With Team"
  team_id = anomaly_team.sched_team.id
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("anomaly_schedule.with_team", "id"),
					resource.TestCheckResourceAttrPair(
						"anomaly_schedule.with_team", "team_id",
						"anomaly_team.sched_team", "id",
					),
				),
			},
		},
	})
}
