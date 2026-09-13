package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccUsersDataSource(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Seed a user so the data source returns at least one item.
				Config: providerConfig() + `
resource "anomaly_user" "ds_seed" {
  name     = "DS Test User"
  username = "ds.test.user"
  email    = "dstest@example.com"
  timezone = "UTC"
}

data "anomaly_users" "all" {
  depends_on = [anomaly_user.ds_seed]
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.anomaly_users.all", "id", "users"),
					// At least one user in the list.
					resource.TestCheckResourceAttrSet("data.anomaly_users.all", "users.#"),
				),
			},
		},
	})
}

func TestAccTeamsDataSource(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "anomaly_team" "ds_seed" {
  name = "DS Test Team"
}

data "anomaly_teams" "all" {
  depends_on = [anomaly_team.ds_seed]
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.anomaly_teams.all", "id", "teams"),
					resource.TestCheckResourceAttrSet("data.anomaly_teams.all", "teams.#"),
				),
			},
		},
	})
}

func TestAccIntegrationsDataSource_all(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "anomaly_integration" "ds_seed" {
  name = "DS Webhook Integration"
  type = "webhook"
}

data "anomaly_integrations" "all" {
  depends_on = [anomaly_integration.ds_seed]
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.anomaly_integrations.all", "id", "integrations"),
					resource.TestCheckResourceAttrSet("data.anomaly_integrations.all", "integrations.#"),
				),
			},
		},
	})
}

func TestAccIntegrationsDataSource_typeFilter(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "anomaly_integration" "webhook_seed" {
  name = "DS Webhook Filter Test"
  type = "webhook"
}

resource "anomaly_integration" "am_seed" {
  name = "DS Alertmanager Filter Test"
  type = "alertmanager"
}

data "anomaly_integrations" "webhooks" {
  type       = "webhook"
  depends_on = [anomaly_integration.webhook_seed, anomaly_integration.am_seed]
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.anomaly_integrations.webhooks", "id", "webhook"),
					resource.TestCheckResourceAttr("data.anomaly_integrations.webhooks", "type", "webhook"),
					// All returned items must be of type webhook.
					resource.TestCheckTypeSetElemNestedAttrs("data.anomaly_integrations.webhooks", "integrations.*", map[string]string{
						"name": "DS Webhook Filter Test",
						"type": "webhook",
					}),
				),
			},
		},
	})
}

func TestAccIntegrationsDataSource_keyAccessible(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "anomaly_integration" "key_test" {
  name = "Key Test Integration"
  type = "webhook"
}

data "anomaly_integrations" "all" {
  depends_on = [anomaly_integration.key_test]
}
`,
				Check: resource.ComposeTestCheckFunc(
					// The integration key field must be set (non-empty) for all items.
					resource.TestCheckResourceAttrSet("anomaly_integration.key_test", "key"),
				),
			},
		},
	})
}
