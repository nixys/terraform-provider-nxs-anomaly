package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func strMap(t *testing.T, m map[string]string) types.Map {
	t.Helper()
	vals := make(map[string]attr.Value, len(m))
	for k, v := range m {
		vals[k] = types.StringValue(v)
	}
	out, diags := types.MapValue(types.StringType, vals)
	if diags.HasError() {
		t.Fatalf("map: %v", diags)
	}
	return out
}

// The API stores header names canonicalised; another spelling would read back
// different from the configuration and fail the apply as inconsistent.
func TestCanonicalHeaderNames(t *testing.T) {
	for name, wantErr := range map[string]bool{
		"X-Api-Key":     false,
		"Authorization": false,
		"x-api-key":     true,
		"X-API-KEY":     true,
	} {
		resp := &validator.MapResponse{}
		canonicalHeaderNames{}.ValidateMap(context.Background(), validator.MapRequest{
			Path: path.Root("headers"), ConfigValue: strMap(t, map[string]string{name: "v"}),
		}, resp)
		if got := resp.Diagnostics.HasError(); got != wantErr {
			t.Errorf("%q: error = %v, want %v", name, got, wantErr)
		}
	}
}

func TestStepsFromPlanSendsHeaders(t *testing.T) {
	step, _ := types.ObjectValue(stepAttrTypes, stepAttrValues(map[string]attr.Value{
		"kind":        types.StringValue("TRIGGER_WEBHOOK"),
		"webhook_url": types.StringValue("https://gw.example.com/hook"),
		"headers":     strMap(t, map[string]string{"X-Api-Key": "env:GW_KEY"}),
	}))
	list, _ := types.ListValue(types.ObjectType{AttrTypes: stepAttrTypes}, []attr.Value{step})
	steps := stepsFromPlan(context.Background(), list)
	h, ok := steps[0].(map[string]any)["headers"].(map[string]any)
	if !ok || h["X-Api-Key"] != "env:GW_KEY" {
		t.Fatalf("headers not sent: %#v", steps[0])
	}
}

func TestStepWithoutHeadersSendsNone(t *testing.T) {
	step, _ := types.ObjectValue(stepAttrTypes, stepAttrValues(map[string]attr.Value{
		"kind": types.StringValue("TRIGGER_WEBHOOK"), "webhook_url": types.StringValue("https://gw.example.com/hook"),
	}))
	list, _ := types.ListValue(types.ObjectType{AttrTypes: stepAttrTypes}, []attr.Value{step})
	if _, ok := stepsFromPlan(context.Background(), list)[0].(map[string]any)["headers"]; ok {
		t.Error("an unset headers map must not be sent")
	}
}

// Absent and empty both read back as null — the value an unset optional
// attribute holds — so a chain without headers plans clean.
func TestHeadersToMap(t *testing.T) {
	if m := headersToMap(nil); !m.IsNull() {
		t.Errorf("absent -> %v, want null", m)
	}
	if m := headersToMap(map[string]any{}); !m.IsNull() {
		t.Errorf("empty -> %v, want null", m)
	}
	m := headersToMap(map[string]any{"Authorization": "env:TOKEN"})
	if v := m.Elements()["Authorization"].(types.String).ValueString(); v != "env:TOKEN" {
		t.Errorf("value = %q", v)
	}
}
