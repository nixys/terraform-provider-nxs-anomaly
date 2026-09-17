package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ── userModelFromAPI ──────────────────────────────────────────────────────────

func TestUserModelFromAPI_Basic(t *testing.T) {
	m := map[string]any{
		"id":                   "usr-1",
		"name":                 "Alice Smith",
		"username":             "alice.smith",
		"email":                "alice@example.com",
		"phone":                "+7999",
		"telegram_id":          "tg-100",
		"timezone":             "Europe/Moscow",
		"on_duty":              true,
		"priority":             "high",
		"notification_targets": []any{},
		"created_at":           "2026-01-01T00:00:00Z",
		"updated_at":           "2026-01-02T00:00:00Z",
	}
	model := userModelFromAPI(context.Background(), m)

	assertEqual(t, "id", "usr-1", model.ID.ValueString())
	assertEqual(t, "name", "Alice Smith", model.Name.ValueString())
	assertEqual(t, "username", "alice.smith", model.Username.ValueString())
	assertEqual(t, "email", "alice@example.com", model.Email.ValueString())
	assertEqual(t, "phone", "+7999", model.Phone.ValueString())
	assertEqual(t, "telegram_id", "tg-100", model.TelegramID.ValueString())
	assertEqual(t, "timezone", "Europe/Moscow", model.Timezone.ValueString())
	if !model.OnDuty.ValueBool() {
		t.Error("on_duty should be true")
	}
	assertEqual(t, "priority", "high", model.Priority.ValueString())
	assertEqual(t, "created_at", "2026-01-01T00:00:00Z", model.CreatedAt.ValueString())
}

func TestUserModelFromAPI_WithNotificationTargets(t *testing.T) {
	m := map[string]any{
		"id":   "usr-2",
		"name": "Bob",
		"notification_targets": []any{
			map[string]any{"channel": "telegram", "target": "123"},
			map[string]any{"channel": "webhook", "target": "https://hook.example.com"},
		},
	}
	model := userModelFromAPI(context.Background(), m)

	elems := model.NotificationTargets.Elements()
	if len(elems) != 2 {
		t.Fatalf("expected 2 notification targets, got %d", len(elems))
	}
	obj0 := elems[0].(types.Object)
	attrs := obj0.Attributes()
	if ch, ok := attrs["channel"].(types.String); !ok || ch.ValueString() != "telegram" {
		t.Errorf("first target channel should be telegram")
	}
	if tgt, ok := attrs["target"].(types.String); !ok || tgt.ValueString() != "123" {
		t.Errorf("first target target should be 123")
	}
}

func TestUserModelFromAPI_EmptyNotificationTargets(t *testing.T) {
	m := map[string]any{"id": "usr-3", "name": "Charlie"}
	model := userModelFromAPI(context.Background(), m)
	if !model.NotificationTargets.IsNull() && len(model.NotificationTargets.Elements()) != 0 {
		t.Errorf("expected empty/null notification_targets, got %d elems", len(model.NotificationTargets.Elements()))
	}
}

// ── teamModelFromAPI ──────────────────────────────────────────────────────────

func TestTeamModelFromAPI(t *testing.T) {
	m := map[string]any{
		"id":         "team-1",
		"name":       "Ops Team",
		"member_ids": []any{"usr-1", "usr-2"},
		"created_at": "2026-01-01T00:00:00Z",
		"updated_at": "2026-01-01T00:00:00Z",
	}
	model := teamModelFromAPI(m)

	assertEqual(t, "id", "team-1", model.ID.ValueString())
	assertEqual(t, "name", "Ops Team", model.Name.ValueString())
	members := model.MemberIDs.Elements()
	if len(members) != 2 {
		t.Fatalf("expected 2 member_ids, got %d", len(members))
	}
	if s, ok := members[0].(types.String); !ok || s.ValueString() != "usr-1" {
		t.Errorf("first member should be usr-1")
	}
}

func TestTeamModelFromAPI_NoMembers(t *testing.T) {
	m := map[string]any{"id": "team-2", "name": "Empty Team"}
	model := teamModelFromAPI(m)
	if len(model.MemberIDs.Elements()) != 0 {
		t.Errorf("expected 0 members, got %d", len(model.MemberIDs.Elements()))
	}
}

// ── scheduleModelFromAPI ──────────────────────────────────────────────────────

func TestScheduleModelFromAPI_WithShifts(t *testing.T) {
	m := map[string]any{
		"id":       "sch-1",
		"name":     "Weekdays",
		"timezone": "UTC",
		"team_id":  "team-1",
		"shifts": []any{
			map[string]any{
				"id":         "shift-1",
				"user_id":    "usr-1",
				"start_at":   "2026-06-01T09:00:00Z",
				"end_at":     "2026-06-01T18:00:00Z",
				"recurrence": "weekly",
			},
		},
		"created_at": "2026-01-01T00:00:00Z",
		"updated_at": "2026-01-01T00:00:00Z",
	}
	model := scheduleModelFromAPI(m)

	assertEqual(t, "id", "sch-1", model.ID.ValueString())
	assertEqual(t, "name", "Weekdays", model.Name.ValueString())
	assertEqual(t, "timezone", "UTC", model.Timezone.ValueString())
	assertEqual(t, "team_id", "team-1", model.TeamID.ValueString())

	shifts := model.Shifts.Elements()
	if len(shifts) != 1 {
		t.Fatalf("expected 1 shift, got %d", len(shifts))
	}
	shiftObj := shifts[0].(types.Object)
	attrs := shiftObj.Attributes()
	assertAttrStr(t, attrs, "id", "shift-1")
	assertAttrStr(t, attrs, "user_id", "usr-1")
	assertAttrStr(t, attrs, "recurrence", "weekly")
}

func TestScheduleModelFromAPI_FormatsShiftTimesInScheduleTimezone(t *testing.T) {
	m := map[string]any{
		"id":       "sch-1",
		"name":     "Moscow",
		"timezone": "Europe/Moscow",
		"shifts": []any{
			map[string]any{
				"id":         "shift-1",
				"user_id":    "usr-1",
				"start_at":   "2026-06-01T06:00:00+00:00",
				"end_at":     "2026-06-01T15:00:00+00:00",
				"recurrence": "weekly",
			},
		},
	}
	model := scheduleModelFromAPI(m)
	attrs := model.Shifts.Elements()[0].(types.Object).Attributes()
	assertAttrStr(t, attrs, "start_at", "2026-06-01T09:00:00+03:00")
	assertAttrStr(t, attrs, "end_at", "2026-06-01T18:00:00+03:00")
}

func TestScheduleModelFromAPI_NoTeam(t *testing.T) {
	m := map[string]any{
		"id":   "sch-2",
		"name": "No Team",
	}
	model := scheduleModelFromAPI(m)
	if !model.TeamID.IsNull() {
		t.Errorf("expected null team_id, got %q", model.TeamID.ValueString())
	}
}

// ── escalationChainModelFromAPI ───────────────────────────────────────────────

func TestEscalationChainModelFromAPI(t *testing.T) {
	m := map[string]any{
		"id":   "esc-1",
		"name": "Critical",
		"steps": []any{
			map[string]any{
				"id":       "step-1",
				"kind":     "NOTIFY_USER",
				"user_ids": []any{"usr-1"},
			},
			map[string]any{
				"id":            "step-2",
				"kind":          "WAIT",
				"delay_minutes": float64(5),
			},
			map[string]any{
				"id":          "step-3",
				"kind":        "TRIGGER_WEBHOOK",
				"webhook_url": "https://hooks.example.com",
			},
		},
		"created_at": "2026-01-01T00:00:00Z",
		"updated_at": "2026-01-01T00:00:00Z",
	}
	model := escalationChainModelFromAPI(m)

	assertEqual(t, "id", "esc-1", model.ID.ValueString())
	assertEqual(t, "name", "Critical", model.Name.ValueString())

	steps := model.Steps.Elements()
	if len(steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(steps))
	}

	step0 := steps[0].(types.Object).Attributes()
	assertAttrStr(t, step0, "kind", "NOTIFY_USER")

	step1 := steps[1].(types.Object).Attributes()
	assertAttrStr(t, step1, "kind", "WAIT")
	if d, ok := step1["delay_minutes"].(types.Int64); !ok || d.ValueInt64() != 5 {
		t.Errorf("step1 delay_minutes should be 5, got %v", step1["delay_minutes"])
	}

	step2 := steps[2].(types.Object).Attributes()
	assertAttrStr(t, step2, "kind", "TRIGGER_WEBHOOK")
	assertAttrStr(t, step2, "webhook_url", "https://hooks.example.com")
}

func TestEscalationChainModelFromAPI_EmptySteps(t *testing.T) {
	m := map[string]any{"id": "esc-2", "name": "Empty"}
	model := escalationChainModelFromAPI(m)
	if len(model.Steps.Elements()) != 0 {
		t.Errorf("expected 0 steps, got %d", len(model.Steps.Elements()))
	}
}

// ── integrationModelFromAPI ───────────────────────────────────────────────────

func TestIntegrationModelFromAPI(t *testing.T) {
	m := map[string]any{
		"id":          "int-1",
		"name":        "Prometheus",
		"key":         "key-abc123",
		"type":        "alertmanager",
		"source_type": "alertmanager",
		"group_by":    []any{"alertname", "cluster"},
		"routes": []any{
			map[string]any{
				"id":                  "route-1",
				"name":                "critical",
				"match_type":          "labels",
				"is_default":          false,
				"labels":              map[string]any{"severity": "critical"},
				"escalation_chain_id": "esc-1",
			},
			map[string]any{
				"id":         "route-2",
				"name":       "default",
				"match_type": "all",
				"is_default": true,
			},
		},
		"created_at": "2026-01-01T00:00:00Z",
		"updated_at": "2026-01-01T00:00:00Z",
	}
	model := integrationModelFromAPI(m)

	assertEqual(t, "id", "int-1", model.ID.ValueString())
	assertEqual(t, "name", "Prometheus", model.Name.ValueString())
	assertEqual(t, "key", "key-abc123", model.Key.ValueString())
	assertEqual(t, "type", "alertmanager", model.Type.ValueString())

	groupBy := model.GroupBy.Elements()
	if len(groupBy) != 2 {
		t.Fatalf("expected 2 group_by, got %d", len(groupBy))
	}

	routes := model.Routes.Elements()
	if len(routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(routes))
	}

	routeObj := routes[0].(types.Object).Attributes()
	assertAttrStr(t, routeObj, "name", "critical")
	assertAttrStr(t, routeObj, "match_type", "labels")
	assertAttrStr(t, routeObj, "escalation_chain_id", "esc-1")

	labelsMap, ok := routeObj["labels"].(types.Map)
	if !ok {
		t.Fatal("labels should be a Map")
	}
	if labelSeverity, ok := labelsMap.Elements()["severity"].(types.String); !ok || labelSeverity.ValueString() != "critical" {
		t.Errorf("labels[severity] should be critical")
	}

	route2 := routes[1].(types.Object).Attributes()
	assertAttrStr(t, route2, "match_type", "all")
	if v, ok := route2["is_default"].(types.Bool); !ok || !v.ValueBool() {
		t.Errorf("route2 is_default should be true")
	}
}

func TestIntegrationModelFromAPI_WebhookSecretNull(t *testing.T) {
	m := map[string]any{"id": "int-2", "name": "Simple"}
	model := integrationModelFromAPI(m)
	if !model.WebhookSecret.IsNull() {
		t.Error("webhook_secret should be null from API response")
	}
}

// ── chatopsChannelModelFromAPI ────────────────────────────────────────────────

func TestChatopsChannelModelFromAPI(t *testing.T) {
	m := map[string]any{
		"id":                    "chat-1",
		"platform":              "telegram",
		"name":                  "ops-alerts",
		"team_id":               "team-1",
		"user_id":               "",
		"commands_enabled":      true,
		"notifications_enabled": false,
		"created_at":            "2026-01-01T00:00:00Z",
		"updated_at":            "2026-01-01T00:00:00Z",
	}
	model := chatopsChannelModelFromAPI(m)

	assertEqual(t, "id", "chat-1", model.ID.ValueString())
	assertEqual(t, "platform", "telegram", model.Platform.ValueString())
	assertEqual(t, "name", "ops-alerts", model.Name.ValueString())
	assertEqual(t, "team_id", "team-1", model.TeamID.ValueString())
	if model.CommandsEnabled.ValueBool() != true {
		t.Error("commands_enabled should be true")
	}
	if model.NotificationsEnabled.ValueBool() != false {
		t.Error("notifications_enabled should be false")
	}
}

func TestChatopsChannelModelFromAPI_NullTeamAndUser(t *testing.T) {
	m := map[string]any{
		"id":       "chat-2",
		"platform": "slack",
		"name":     "dev",
	}
	model := chatopsChannelModelFromAPI(m)
	if !model.TeamID.IsNull() {
		t.Errorf("team_id should be null, got %q", model.TeamID.ValueString())
	}
	if !model.UserID.IsNull() {
		t.Errorf("user_id should be null, got %q", model.UserID.ValueString())
	}
}

// ── Plan-to-API conversion functions ─────────────────────────────────────────

func TestStringListToAny(t *testing.T) {
	ctx := context.Background()
	list, _ := types.ListValueFrom(ctx, types.StringType, []string{"a", "b", "c"})

	result := stringListToAny(ctx, list)
	if len(result) != 3 {
		t.Fatalf("expected 3 items, got %d", len(result))
	}
	for i, want := range []string{"a", "b", "c"} {
		if result[i] != want {
			t.Errorf("item[%d] = %v; want %q", i, result[i], want)
		}
	}
}

func TestStringListToAny_Null(t *testing.T) {
	result := stringListToAny(context.Background(), types.ListNull(types.StringType))
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestNotificationTargetsFromPlan(t *testing.T) {
	obj0, _ := types.ObjectValue(notificationTargetAttrTypes, map[string]attr.Value{
		"channel": types.StringValue("telegram"),
		"target":  types.StringValue("12345"),
	})
	obj1, _ := types.ObjectValue(notificationTargetAttrTypes, map[string]attr.Value{
		"channel": types.StringValue("webhook"),
		"target":  types.StringValue("https://example.com"),
	})
	list, _ := types.ListValue(types.ObjectType{AttrTypes: notificationTargetAttrTypes}, []attr.Value{obj0, obj1})

	result := notificationTargetsFromPlan(list)
	if len(result) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(result))
	}
	if result[0]["type"] != "telegram" {
		t.Errorf("target[0].type = %v; want telegram", result[0]["type"])
	}
	if result[1]["target"] != "https://example.com" {
		t.Errorf("target[1].target = %v; want https://example.com", result[1]["target"])
	}
}

func TestNotificationTargetsFromPlan_Null(t *testing.T) {
	result := notificationTargetsFromPlan(types.ListNull(types.StringType))
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestStringSliceToList(t *testing.T) {
	list := stringSliceToList([]string{"x", "y"})
	elems := list.Elements()
	if len(elems) != 2 {
		t.Fatalf("expected 2, got %d", len(elems))
	}
	s, ok := elems[0].(types.String)
	if !ok || s.ValueString() != "x" {
		t.Errorf("first element should be x")
	}
}

func TestStringSliceToList_Nil(t *testing.T) {
	list := stringSliceToList(nil)
	if len(list.Elements()) != 0 {
		t.Errorf("expected 0 elements, got %d", len(list.Elements()))
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func assertEqual(t *testing.T, field, want, got string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %q; want %q", field, got, want)
	}
}

func assertAttrStr(t *testing.T, attrs map[string]attr.Value, key, want string) {
	t.Helper()
	v, ok := attrs[key]
	if !ok {
		t.Errorf("key %q not found in attrs", key)
		return
	}
	s, ok := v.(types.String)
	if !ok {
		t.Errorf("attrs[%q] is not types.String (got %T)", key, v)
		return
	}
	if s.ValueString() != want {
		t.Errorf("attrs[%q] = %q; want %q", key, s.ValueString(), want)
	}
}

// Unset on_duty must not be sent: responders take and hand over duty through
// the API, and sending false on every apply took them off duty.
func TestUserBodyLeavesUnsetOnDutyAlone(t *testing.T) {
	plan := userModel{OnDuty: types.BoolNull(), Username: types.StringNull(), NotificationPolicies: types.ObjectNull(nil)}
	if _, sent := userBody(plan)["on_duty"]; sent {
		t.Error("on_duty sent although it is not in the configuration")
	}
	plan.OnDuty = types.BoolValue(true)
	if got := userBody(plan)["on_duty"]; got != true {
		t.Errorf("on_duty = %v, want true when configured", got)
	}
}
