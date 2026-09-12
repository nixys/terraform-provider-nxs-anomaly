package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccChatopsChannelResource_lifecycle(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create
			{
				Config: providerConfig() + `
resource "anomaly_team" "chat_team" {
  name = "ChatOps Team TF"
}

resource "anomaly_chatops_channel" "test" {
  platform              = "telegram"
  name                  = "ops-alerts-tf"
  team_id               = anomaly_team.chat_team.id
  commands_enabled      = true
  notifications_enabled = true
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("anomaly_chatops_channel.test", "id"),
					resource.TestCheckResourceAttr("anomaly_chatops_channel.test", "platform", "telegram"),
					resource.TestCheckResourceAttr("anomaly_chatops_channel.test", "name", "ops-alerts-tf"),
					resource.TestCheckResourceAttr("anomaly_chatops_channel.test", "commands_enabled", "true"),
					resource.TestCheckResourceAttr("anomaly_chatops_channel.test", "notifications_enabled", "true"),
					resource.TestCheckResourceAttrPair(
						"anomaly_chatops_channel.test", "team_id",
						"anomaly_team.chat_team", "id",
					),
				),
			},
			// Update: disable notifications
			{
				Config: providerConfig() + `
resource "anomaly_team" "chat_team" {
  name = "ChatOps Team TF"
}

resource "anomaly_chatops_channel" "test" {
  platform              = "telegram"
  name                  = "ops-alerts-tf"
  team_id               = anomaly_team.chat_team.id
  commands_enabled      = true
  notifications_enabled = false
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("anomaly_chatops_channel.test", "notifications_enabled", "false"),
				),
			},
		},
	})
}

func TestAccChatopsChannelResource_noTeam(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "anomaly_chatops_channel" "direct" {
  platform = "slack"
  name     = "direct-alerts-tf"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("anomaly_chatops_channel.direct", "id"),
					resource.TestCheckResourceAttr("anomaly_chatops_channel.direct", "platform", "slack"),
				),
			},
		},
	})
}
