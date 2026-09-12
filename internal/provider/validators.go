package provider

// validPriorities, validStepKinds, etc. are referenced from schema Validators.
// The actual validator instances are built inline in each schema using
// github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator.

var validPriorities = []string{"low", "medium", "high"}

var validUserRoles = []string{"", "viewer", "responder", "editor", "admin"}

var validStepKinds = []string{
	"WAIT", "NOTIFY_USER", "NOTIFY_SCHEDULE", "NOTIFY_TEAM",
	"NOTIFY_EMERGENCY", "NOTIFY_DUTY_USERS", "TRIGGER_WEBHOOK",
	"CREATE_ISSUE", "RESOLVE", "REPEAT",
}

var validIntegrationTypes = []string{
	"webhook", "alertmanager", "pagerduty", "victorops", "grafana-alerting",
}

var validChatopsPlatforms = []string{"telegram", "slack", "mattermost"}

var validShiftRecurrences = []string{"none", "daily", "weekly"}

var validRouteMatchTypes = []string{"all", "labels", "regex"}
