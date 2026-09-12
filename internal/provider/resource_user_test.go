package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccUserResource_lifecycle(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create
			{
				Config: providerConfig() + `
resource "anomaly_user" "test" {
  name     = "Test User TF"
  username = "testuser.tf"
  email    = "testuser@example.com"
  timezone = "UTC"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("anomaly_user.test", "id"),
					resource.TestCheckResourceAttr("anomaly_user.test", "name", "Test User TF"),
					resource.TestCheckResourceAttr("anomaly_user.test", "username", "testuser.tf"),
					resource.TestCheckResourceAttr("anomaly_user.test", "email", "testuser@example.com"),
					resource.TestCheckResourceAttr("anomaly_user.test", "timezone", "UTC"),
					resource.TestCheckResourceAttrSet("anomaly_user.test", "created_at"),
				),
			},
			// ImportState
			{
				ResourceName:      "anomaly_user.test",
				ImportState:       true,
				ImportStateVerify: false, // API doesn't return username if auto-generated
			},
			// Update
			{
				Config: providerConfig() + `
resource "anomaly_user" "test" {
  name     = "Test User TF Updated"
  username = "testuser.tf"
  email    = "updated@example.com"
  timezone = "Europe/Moscow"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("anomaly_user.test", "name", "Test User TF Updated"),
					resource.TestCheckResourceAttr("anomaly_user.test", "email", "updated@example.com"),
					resource.TestCheckResourceAttr("anomaly_user.test", "timezone", "Europe/Moscow"),
				),
			},
		},
	})
}

func TestAccUserResource_withNotificationTargets(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "anomaly_user" "notif" {
  name = "Notification User"
  notification_targets = [
    {
      channel = "webhook"
      target  = "https://hooks.example.com/notify"
    }
  ]
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("anomaly_user.notif", "id"),
					resource.TestCheckResourceAttr("anomaly_user.notif", "notification_targets.#", "1"),
					resource.TestCheckResourceAttr("anomaly_user.notif", "notification_targets.0.channel", "webhook"),
					resource.TestCheckResourceAttr("anomaly_user.notif", "notification_targets.0.target", "https://hooks.example.com/notify"),
				),
			},
		},
	})
}

func TestAccUserResource_disappears(t *testing.T) {
	testAccPreCheck(t)

	var userID string

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "anomaly_user" "disappear" {
  name = "Disappear Test User"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("anomaly_user.disappear", "id"),
					// Capture ID for use in next step.
					resource.TestCheckResourceAttrWith("anomaly_user.disappear", "id", func(v string) error {
						userID = v
						return nil
					}),
				),
			},
			// Re-plan after external deletion should recreate the resource.
			{
				PreConfig: func() {
					// Delete the user externally via API.
					if userID != "" {
						c := newTestClient(t)
						if err := c.delete(testCtx(t), fmt.Sprintf("/api/v1/users/%s", userID)); err != nil {
							t.Fatal(err)
						}
					}
				},
				Config: providerConfig() + `
resource "anomaly_user" "disappear" {
  name = "Disappear Test User"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("anomaly_user.disappear", "id"),
					resource.TestCheckResourceAttrWith("anomaly_user.disappear", "id", func(v string) error {
						if v == userID {
							return fmt.Errorf("deleted user was not recreated")
						}
						return nil
					}),
				),
			},
		},
	})
}
