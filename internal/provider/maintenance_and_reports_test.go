package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// Compile-time check that the new resource can be imported like the others.
var _ resource.ResourceWithImportState = &MaintenanceWindowResource{}

// ── maintenance windows ───────────────────────────────────────────────────────

func TestMaintenanceWindowModelFromAPI(t *testing.T) {
	model := maintenanceWindowModelFromAPI(map[string]any{
		"id": "mnt-1", "name": "DB upgrade", "reason": "planned",
		"team_id":         "team-1",
		"integration_ids": []any{"int-1", "int-2"},
		"starts_at":       "2026-06-01T06:00:00+00:00",
		"ends_at":         "2026-06-01T08:00:00+00:00",
		"created_at":      "2026-05-01T00:00:00+00:00",
		"updated_at":      "2026-05-01T00:00:00+00:00",
	})
	assertEqual(t, "id", "mnt-1", model.ID.ValueString())
	assertEqual(t, "team_id", "team-1", model.TeamID.ValueString())
	assertEqual(t, "starts_at", "2026-06-01T06:00:00+00:00", model.StartsAt.ValueString())
	if len(model.IntegrationIDs.Elements()) != 2 {
		t.Fatalf("integration_ids were not mapped: %#v", model.IntegrationIDs)
	}
}

func TestMaintenanceWindowModelFromAPI_TeamIDNullWhenUnowned(t *testing.T) {
	model := maintenanceWindowModelFromAPI(map[string]any{"id": "mnt-2", "name": "no team"})
	if !model.TeamID.IsNull() {
		t.Errorf("team_id = %q; want null", model.TeamID.ValueString())
	}
}

func TestMaintenanceWindowBody(t *testing.T) {
	body := maintenanceWindowBody(context.Background(), maintenanceWindowModel{
		Name:           types.StringValue("DB upgrade"),
		IntegrationIDs: stringSliceToList([]string{"int-1"}),
		StartsAt:       types.StringValue("2026-06-01T09:00:00+03:00"),
		EndsAt:         types.StringValue("2026-06-01T11:00:00+03:00"),
		Reason:         types.StringNull(),
		TeamID:         types.StringNull(),
	})
	if body["name"] != "DB upgrade" || body["starts_at"] != "2026-06-01T09:00:00+03:00" {
		t.Fatalf("body mismatch: %#v", body)
	}
	if len(body["integration_ids"].([]any)) != 1 {
		t.Fatalf("integration_ids missing from body: %#v", body)
	}
	// Unset optionals stay out of the payload so the API keeps its own value.
	if _, ok := body["reason"]; ok {
		t.Error("null reason should not be sent")
	}
	if _, ok := body["team_id"]; ok {
		t.Error("null team_id should not be sent")
	}
}

// The engine answers in UTC. A window written with an offset must not come back
// as a different string for the same instant, or every plan shows drift.
func TestKeepEquivalentBoundsPreservesConfiguredOffset(t *testing.T) {
	config := maintenanceWindowModel{
		StartsAt: types.StringValue("2026-06-01T09:00:00+03:00"),
		EndsAt:   types.StringValue("2026-06-01T11:00:00+03:00"),
	}
	model := maintenanceWindowModelFromAPI(map[string]any{
		"starts_at": "2026-06-01T06:00:00+00:00",
		"ends_at":   "2026-06-01T08:00:00+00:00",
	})
	keepEquivalentBounds(&model, config)
	assertEqual(t, "starts_at", "2026-06-01T09:00:00+03:00", model.StartsAt.ValueString())
	assertEqual(t, "ends_at", "2026-06-01T11:00:00+03:00", model.EndsAt.ValueString())
}

func TestKeepEquivalentBoundsReportsRealDrift(t *testing.T) {
	config := maintenanceWindowModel{
		StartsAt: types.StringValue("2026-06-01T09:00:00+03:00"),
		EndsAt:   types.StringValue("2026-06-01T11:00:00+03:00"),
	}
	model := maintenanceWindowModelFromAPI(map[string]any{
		"starts_at": "2026-06-02T06:00:00+00:00",
		"ends_at":   "2026-06-01T08:00:00+00:00",
	})
	keepEquivalentBounds(&model, config)
	assertEqual(t, "starts_at", "2026-06-02T06:00:00+00:00", model.StartsAt.ValueString())
}

// ── integration heartbeat ─────────────────────────────────────────────────────

func TestIntegrationModelIncludesHeartbeat(t *testing.T) {
	model := integrationModelFromAPI(map[string]any{
		"id": "int-1", "name": "AM",
		"heartbeat": map[string]any{"interval_seconds": float64(300), "grace_seconds": float64(100)},
	})
	if model.Heartbeat.IsNull() {
		t.Fatal("heartbeat was not mapped")
	}
	a := model.Heartbeat.Attributes()
	if attrInt64(a, "interval_seconds", 0) != 300 || attrInt64(a, "grace_seconds", 0) != 100 {
		t.Fatalf("heartbeat values mismatch: %#v", a)
	}
}

func TestIntegrationBodySendsHeartbeat(t *testing.T) {
	heartbeat, _ := types.ObjectValue(heartbeatAttrTypes, map[string]attr.Value{
		"interval_seconds": types.Int64Value(600),
		"grace_seconds":    types.Int64Value(0),
	})
	body := integrationBody(context.Background(), integrationModel{
		Name: types.StringValue("AM"), Type: types.StringValue("alertmanager"),
		Heartbeat: heartbeat,
	})
	hb, ok := body["heartbeat"].(map[string]any)
	if !ok {
		t.Fatalf("heartbeat missing from body: %#v", body)
	}
	if hb["interval_seconds"] != int64(600) || hb["grace_seconds"] != int64(0) {
		t.Fatalf("heartbeat payload mismatch: %#v", hb)
	}
}

func TestIntegrationBodyOmitsUnsetHeartbeat(t *testing.T) {
	body := integrationBody(context.Background(), integrationModel{
		Name: types.StringValue("AM"), Type: types.StringValue("webhook"),
		Heartbeat: types.ObjectNull(heartbeatAttrTypes),
	})
	// The API keys off the presence of the field, so omitting it leaves a
	// heartbeat configured elsewhere alone instead of silently clearing it.
	if _, ok := body["heartbeat"]; ok {
		t.Errorf("null heartbeat should not be sent: %#v", body)
	}
}

// ── escalation step allow_uncovered ───────────────────────────────────────────

func TestStepsFromPlanSendsAllowUncovered(t *testing.T) {
	step, _ := types.ObjectValue(stepAttrTypes, stepAttrValues(map[string]attr.Value{
		"kind":            types.StringValue("NOTIFY_SCHEDULE"),
		"schedule_id":     types.StringValue("sch-1"),
		"allow_uncovered": types.BoolValue(true),
	}))
	list, _ := types.ListValue(types.ObjectType{AttrTypes: stepAttrTypes}, []attr.Value{step})

	steps := stepsFromPlan(context.Background(), list)
	if len(steps) != 1 {
		t.Fatalf("expected one step, got %#v", steps)
	}
	m := steps[0].(map[string]any)
	if m["allow_uncovered"] != true {
		t.Fatalf("allow_uncovered was not sent: %#v", m)
	}
}

func TestEscalationChainModelMapsAllowUncovered(t *testing.T) {
	model := escalationChainModelFromAPI(map[string]any{
		"id": "chain-1", "name": "primary",
		"steps": []any{map[string]any{
			"id": "step-1", "kind": "NOTIFY_SCHEDULE",
			"schedule_id": "sch-1", "allow_uncovered": true,
		}},
	})
	elems := model.Steps.Elements()
	if len(elems) != 1 {
		t.Fatalf("expected one step, got %#v", elems)
	}
	a := elems[0].(types.Object).Attributes()
	if !attrBool(a, "allow_uncovered", false) {
		t.Fatalf("allow_uncovered was not mapped: %#v", a)
	}
}

// stepAttrValues fills every step attribute with a null of the right type and
// then applies the overrides, so tests only state the fields they care about.
func stepAttrValues(overrides map[string]attr.Value) map[string]attr.Value {
	values := make(map[string]attr.Value, len(stepAttrTypes))
	for name, t := range stepAttrTypes {
		values[name] = nullValueOf(t)
	}
	for name, v := range overrides {
		values[name] = v
	}
	return values
}

func nullValueOf(t attr.Type) attr.Value {
	switch typed := t.(type) {
	case basetypes.StringType:
		return types.StringNull()
	case basetypes.BoolType:
		return types.BoolNull()
	case basetypes.Int64Type:
		return types.Int64Null()
	case types.ListType:
		return types.ListNull(typed.ElemType)
	case types.MapType:
		return types.MapNull(typed.ElemType)
	}
	return nil
}
