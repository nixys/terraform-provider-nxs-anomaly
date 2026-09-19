package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccIntegrationResource_lifecycle(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with a chain and route
			{
				Config: providerConfig() + `
resource "anomaly_escalation_chain" "int_chain" {
  name = "Integration Chain TF"
}

resource "anomaly_integration" "test" {
  name     = "Prometheus TF"
  type     = "alertmanager"
  group_by = ["alertname", "cluster"]

  routes = [
    {
      name                = "default"
      escalation_chain_id = anomaly_escalation_chain.int_chain.id
      is_default          = true
    }
  ]
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("anomaly_integration.test", "id"),
					resource.TestCheckResourceAttrSet("anomaly_integration.test", "key"),
					resource.TestCheckResourceAttr("anomaly_integration.test", "name", "Prometheus TF"),
					resource.TestCheckResourceAttr("anomaly_integration.test", "type", "alertmanager"),
					resource.TestCheckResourceAttr("anomaly_integration.test", "group_by.#", "2"),
					resource.TestCheckResourceAttr("anomaly_integration.test", "routes.#", "1"),
					resource.TestCheckResourceAttr("anomaly_integration.test", "routes.0.name", "default"),
				),
			},
			// Update: rename + add condition
			{
				Config: providerConfig() + `
resource "anomaly_escalation_chain" "int_chain" {
  name = "Integration Chain TF"
}

resource "anomaly_integration" "test" {
  name     = "Prometheus TF Updated"
  type     = "alertmanager"
  group_by = ["alertname"]

  routes = [
    {
      name                = "critical"
      escalation_chain_id = anomaly_escalation_chain.int_chain.id
      match_type = "labels"
      labels = { severity = "critical" }
    },
    {
      name = "default"
      is_default = true
      escalation_chain_id = anomaly_escalation_chain.int_chain.id
    }
  ]
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("anomaly_integration.test", "name", "Prometheus TF Updated"),
					resource.TestCheckResourceAttr("anomaly_integration.test", "routes.0.name", "critical"),
					resource.TestCheckResourceAttr("anomaly_integration.test", "routes.0.labels.%", "1"),
					resource.TestCheckResourceAttr("anomaly_integration.test", "routes.0.labels.severity", "critical"),
				),
			},
		},
	})
}

func TestAccIntegrationResource_keyIsStable(t *testing.T) {
	testAccPreCheck(t)

	var firstKey string

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "anomaly_integration" "stable_key" {
  name = "Stable Key Integration"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("anomaly_integration.stable_key", "key", func(v string) error {
						firstKey = v
						return nil
					}),
				),
			},
			// Re-apply same config: key must not change.
			{
				Config: providerConfig() + `
resource "anomaly_integration" "stable_key" {
  name = "Stable Key Integration"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("anomaly_integration.stable_key", "key", func(v string) error {
						if v != firstKey {
							return fmt.Errorf("key changed: %q → %q", firstKey, v)
						}
						return nil
					}),
				),
			},
		},
	})
}

func TestAccIntegrationResource_webhookSecret(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "anomaly_integration" "secret_test" {
  name           = "Secret Integration TF"
  webhook_secret = "super-secret-value"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("anomaly_integration.secret_test", "id"),
					// Secret is stored in state but not exposed in plan output.
					resource.TestCheckResourceAttr("anomaly_integration.secret_test", "webhook_secret", "super-secret-value"),
				),
			},
		},
	})
}

// The API normalises heartbeat values (grace 0 → interval/3, interval below 60
// → 60). Spelled the other way — the module example passes grace_seconds = 0 —
// apply failed with "inconsistent result", tainted the integration, and every
// later apply replaced it with a new routing key.
func TestAccIntegrationResource_heartbeatAsWritten(t *testing.T) {
	testAccPreCheck(t)

	config := providerConfig() + `
resource "anomaly_integration" "hb" {
  name      = "Heartbeat As Written"
  heartbeat = { interval_seconds = 30, grace_seconds = 0 }
}
`
	var firstKey string
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("anomaly_integration.hb", "heartbeat.interval_seconds", "30"),
					resource.TestCheckResourceAttr("anomaly_integration.hb", "heartbeat.grace_seconds", "0"),
					resource.TestCheckResourceAttrWith("anomaly_integration.hb", "key", func(v string) error {
						firstKey = v
						return nil
					}),
				),
			},
			{
				Config:   config,
				PlanOnly: true,
			},
			{
				Config: config,
				Check: resource.TestCheckResourceAttrWith("anomaly_integration.hb", "key", func(v string) error {
					if v != firstKey {
						return fmt.Errorf("the integration was replaced: key %q → %q", firstKey, v)
					}
					return nil
				}),
			},
		},
	})
}
