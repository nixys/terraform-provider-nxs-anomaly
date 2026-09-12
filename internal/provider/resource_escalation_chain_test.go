package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccEscalationChainResource_lifecycle(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with steps
			{
				Config: providerConfig() + `
resource "anomaly_user" "esc_user" {
  name = "Escalation Test User"
}

resource "anomaly_escalation_chain" "test" {
  name = "TF Test Chain"
  steps = [
    {
      kind     = "notify_user"
      user_ids = [anomaly_user.esc_user.id]
    },
    {
      kind             = "wait"
      delay_minutes = 2
    },
    {
      kind        = "trigger_webhook"
      webhook_url = "https://hooks.example.com/alert"
    }
  ]
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("anomaly_escalation_chain.test", "id"),
					resource.TestCheckResourceAttr("anomaly_escalation_chain.test", "name", "TF Test Chain"),
					resource.TestCheckResourceAttr("anomaly_escalation_chain.test", "steps.#", "3"),
					resource.TestCheckResourceAttr("anomaly_escalation_chain.test", "steps.0.kind", "notify_user"),
					resource.TestCheckResourceAttr("anomaly_escalation_chain.test", "steps.1.kind", "wait"),
					resource.TestCheckResourceAttr("anomaly_escalation_chain.test", "steps.1.delay_minutes", "2"),
					resource.TestCheckResourceAttr("anomaly_escalation_chain.test", "steps.2.kind", "trigger_webhook"),
					resource.TestCheckResourceAttr("anomaly_escalation_chain.test", "steps.2.webhook_url", "https://hooks.example.com/alert"),
				),
			},
			// Update: rename and simplify steps
			{
				Config: providerConfig() + `
resource "anomaly_user" "esc_user" {
  name = "Escalation Test User"
}

resource "anomaly_escalation_chain" "test" {
  name = "TF Test Chain Updated"
  steps = [
    {
      kind             = "wait"
      delay_minutes = 5
    }
  ]
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("anomaly_escalation_chain.test", "name", "TF Test Chain Updated"),
					resource.TestCheckResourceAttr("anomaly_escalation_chain.test", "steps.#", "1"),
					resource.TestCheckResourceAttr("anomaly_escalation_chain.test", "steps.0.kind", "wait"),
					resource.TestCheckResourceAttr("anomaly_escalation_chain.test", "steps.0.delay_minutes", "5"),
				),
			},
		},
	})
}

func TestAccEscalationChainResource_empty(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "anomaly_escalation_chain" "empty" {
  name = "Empty Chain"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("anomaly_escalation_chain.empty", "id"),
					resource.TestCheckResourceAttr("anomaly_escalation_chain.empty", "name", "Empty Chain"),
				),
			},
		},
	})
}
