package provider_test

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/nixys/terraform-provider-nxs-anomaly/internal/provider"
)

// testAccProtoV6ProviderFactories wires up the provider for acceptance tests.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"anomaly": providerserver.NewProtocol6WithError(provider.New("test")()),
}

// testAccPreCheck skips the test if NXS_ANOMALY_URL is not set.
// Acceptance tests require a live nxs-anomaly instance.
func testAccPreCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("NXS_ANOMALY_URL") == "" {
		t.Skip("NXS_ANOMALY_URL not set — skipping acceptance test")
	}
}

// providerConfig returns the provider block for acceptance tests using env vars.
func providerConfig() string {
	return `
provider "anomaly" {}
`
}

// TestAccProvider_basic verifies the provider can be configured and a plan succeeds.
func TestAccProvider_basic(t *testing.T) {
	testAccPreCheck(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "anomaly_user" "probe" {
  name = "Provider Probe User"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("anomaly_user.probe", "name", "Provider Probe User"),
					resource.TestCheckResourceAttrSet("anomaly_user.probe", "id"),
				),
			},
		},
	})
}
