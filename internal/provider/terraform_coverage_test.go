package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

func TestProviderSchemaHasNoDiagnostics(t *testing.T) {
	server := providerserver.NewProtocol6(New("test")())()
	response, err := server.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatalf("get provider schema: %v", err)
	}
	for _, diagnostic := range response.Diagnostics {
		if diagnostic.Severity == tfprotov6.DiagnosticSeverityError {
			t.Errorf("provider schema error: %s: %s", diagnostic.Summary, diagnostic.Detail)
		}
	}
}

// TestProviderExposesExpectedSurface pins the set of type names the provider
// serves. The point is the direction it fails in: a CRUD collection the API
// grew and nobody wired up (maintenance windows went unnoticed for exactly this
// reason), or a name quietly dropped from registration — both show up here as a
// diff instead of as a practitioner's "unsupported resource type".
func TestProviderExposesExpectedSurface(t *testing.T) {
	server := providerserver.NewProtocol6(New("test")())()
	response, err := server.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatalf("get provider schema: %v", err)
	}

	wantResources := []string{
		"anomaly_chatops_channel",
		"anomaly_escalation_chain",
		"anomaly_integration",
		"anomaly_maintenance_window",
		"anomaly_schedule",
		"anomaly_schedule_override",
		"anomaly_team",
		"anomaly_user",
	}
	wantDataSources := []string{
		"anomaly_chatops_channel", "anomaly_chatops_channels",
		"anomaly_escalation_chain", "anomaly_escalation_chains",
		"anomaly_integration", "anomaly_integrations",
		"anomaly_maintenance_window", "anomaly_maintenance_windows",
		"anomaly_on_call",
		"anomaly_readiness",
		"anomaly_schedule", "anomaly_schedules",
		"anomaly_schedule_coverage", "anomaly_schedule_preview",
		"anomaly_team", "anomaly_teams",
		"anomaly_user", "anomaly_users",
	}

	assertNames(t, "resource", wantResources, response.ResourceSchemas)
	assertNames(t, "data source", wantDataSources, response.DataSourceSchemas)
}

func assertNames[T any](t *testing.T, kind string, want []string, got map[string]T) {
	t.Helper()
	for _, name := range want {
		if _, ok := got[name]; !ok {
			t.Errorf("%s %q is not registered", kind, name)
		}
	}
	for name := range got {
		if !contains(want, name) {
			t.Errorf("%s %q is registered but not in the expected set — add it here on purpose", kind, name)
		}
	}
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func TestScheduleModelFromAPIIncludesV2Fields(t *testing.T) {
	model := scheduleModelFromAPI(map[string]any{
		"id": "sch-1", "name": "Primary", "timezone": "Europe/Moscow",
		"enabled": false, "notify_on_shift_change": true,
		"rotation": map[string]any{
			"enabled": true, "start_at": "2026-06-01T06:00:00+00:00",
			"handoff_interval": float64(2), "handoff_unit": "weeks",
			"participant_ids": []any{"usr-1", "usr-2"},
			"restriction":     map[string]any{"start": "09:00", "end": "18:00", "days": []any{"mon", "tue"}},
		},
	})
	if model.Enabled.ValueBool() || !model.NotifyOnShiftChange.ValueBool() {
		t.Fatalf("schedule flags were not mapped: %#v", model)
	}
	a := model.Rotation.Attributes()
	if attrStr(a, "start_at") != "2026-06-01T09:00:00+03:00" || attrInt64(a, "handoff_interval", 0) != 2 {
		t.Fatalf("rotation was not mapped: %#v", a)
	}
}

func TestScheduleOverrideModelFromAPIUsesScheduleTimezone(t *testing.T) {
	model := scheduleOverrideModelFromAPI(map[string]any{
		"id": "ovr-1", "user_id": "usr-1", "start_at": "2026-06-01T06:00:00Z",
		"until": "2026-06-01T15:00:00Z", "reason": "cover",
	}, map[string]any{"timezone": "Europe/Moscow"}, "sch-1")
	if model.StartAt.ValueString() != "2026-06-01T09:00:00+03:00" || model.Until.ValueString() != "2026-06-01T18:00:00+03:00" {
		t.Fatalf("override times were not normalized: %#v", model)
	}
}

func TestUserModelAndBodyIncludeRoleAndPolicies(t *testing.T) {
	model := userModelFromAPI(context.Background(), map[string]any{
		"id": "usr-1", "name": "Alice", "username": "alice", "role": "editor",
		"notification_policies": map[string]any{"default": []any{
			map[string]any{"channel": "email", "target": "alice@example.test", "wait_minutes": float64(5)},
		}},
	})
	if model.Role.ValueString() != "editor" || model.NotificationPolicies.IsNull() {
		t.Fatalf("user fields were not mapped: %#v", model)
	}
	body := userBody(model)
	if body["role"] != "editor" || body["notification_policies"] == nil {
		t.Fatalf("user fields were not sent: %#v", body)
	}
}

func TestIntegrationModelIncludesOwnershipKafkaAndLegacyPool(t *testing.T) {
	model := integrationModelFromAPI(map[string]any{
		"id": "int-1", "name": "AM", "team_id": "team-1", "kafka_topic": "alerts",
		"legacy_pool": map[string]any{"name": "pool", "emails": []any{"ops@example.test"}},
	})
	if model.TeamID.ValueString() != "team-1" || model.KafkaTopic.ValueString() != "alerts" || model.LegacyPool.IsNull() {
		t.Fatalf("integration fields were not mapped: %#v", model)
	}
}

func TestChatopsModelIncludesDeliveryFields(t *testing.T) {
	model := chatopsChannelModelFromAPI(map[string]any{
		"id": "chat-1", "platform": "slack", "name": "ops",
		"webhook_url": "env:NXS_CHATOPS_WEBHOOK", "external_id": "C123",
	})
	if model.WebhookURL.ValueString() != "env:NXS_CHATOPS_WEBHOOK" || model.ExternalID.ValueString() != "C123" {
		t.Fatalf("ChatOps delivery fields were not mapped: %#v", model)
	}
}

func TestRotationFromPlan(t *testing.T) {
	restriction, _ := types.ObjectValue(rotationRestrictionAttrTypes, map[string]attr.Value{
		"start": types.StringValue("09:00"), "end": types.StringValue("18:00"),
		"days": stringSliceToList([]string{"mon"}),
	})
	rotation, _ := types.ObjectValue(rotationAttrTypes, map[string]attr.Value{
		"enabled": types.BoolValue(true), "start_at": types.StringValue("2026-06-01T09:00:00+03:00"),
		"handoff_interval": types.Int64Value(1), "handoff_unit": types.StringValue("weeks"),
		"participant_ids": stringSliceToList([]string{"usr-1"}), "restriction": restriction,
	})
	body := rotationFromPlan(context.Background(), rotation)
	if body["handoff_unit"] != "weeks" || len(body["participant_ids"].([]any)) != 1 {
		t.Fatalf("rotation payload mismatch: %#v", body)
	}
}
