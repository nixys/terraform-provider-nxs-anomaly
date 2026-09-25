package provider_test

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// Outbound headers (nxs-anomaly 1.7.0) on a TRIGGER_WEBHOOK step: written,
// read back without drift, imported, and cleared when removed.
func TestAccEscalationChainResource_headers(t *testing.T) {
	testAccPreCheck(t)
	chain := func(headers string) string {
		return providerConfig() + `
resource "anomaly_escalation_chain" "hdr" {
  name  = "Headers chain TF"
  steps = [{
    kind        = "TRIGGER_WEBHOOK"
    webhook_url = "https://gw.example.com/hook"
` + headers + `
  }]
}
`
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      chain(`    headers = { "x-api-key" = "k" }`),
				ExpectError: regexp.MustCompile(`canonical form`),
			},
			{
				Config: chain(`    headers = { "X-Api-Key" = "inline-key-tf", "Authorization" = "env:NXS_TF_ACC_GW_TOKEN" }`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("anomaly_escalation_chain.hdr", "steps.0.headers.%", "2"),
					resource.TestCheckResourceAttr("anomaly_escalation_chain.hdr", "steps.0.headers.X-Api-Key", "inline-key-tf"),
					resource.TestCheckResourceAttr("anomaly_escalation_chain.hdr", "steps.0.headers.Authorization", "env:NXS_TF_ACC_GW_TOKEN"),
				),
			},
			{
				ResourceName:      "anomaly_escalation_chain.hdr",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: chain(""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("anomaly_escalation_chain.hdr", "steps.0.headers.%"),
				),
			},
			// The import after removal proves the API holds no headers either.
			{
				ResourceName:      "anomaly_escalation_chain.hdr",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// The same on a ChatOps channel, whose update merges: removing headers from the
// configuration has to clear them on the server, not leave them in place.
func TestAccChatopsChannelResource_headers(t *testing.T) {
	testAccPreCheck(t)
	channel := func(headers string) string {
		return providerConfig() + `
resource "anomaly_chatops_channel" "hdr" {
  platform    = "slack"
  name        = "headers-channel-tf"
  webhook_url = "https://hooks.example.com/services/tf"
` + headers + `
}
`
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: channel(`  headers = { "X-Api-Key" = "inline-key-tf" }`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("anomaly_chatops_channel.hdr", "headers.%", "1"),
					resource.TestCheckResourceAttr("anomaly_chatops_channel.hdr", "headers.X-Api-Key", "inline-key-tf"),
				),
			},
			{
				ResourceName:      "anomaly_chatops_channel.hdr",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: channel(""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("anomaly_chatops_channel.hdr", "headers.%"),
				),
			},
			{
				ResourceName:      "anomaly_chatops_channel.hdr",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
