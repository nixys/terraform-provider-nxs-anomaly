package provider

import (
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"testing"
)

func TestKnownPlanMatchesState(t *testing.T) {
	stringValue := func(v any) tftypes.Value { return tftypes.NewValue(tftypes.String, v) }
	typ := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"name": tftypes.String, "computed": tftypes.String}}
	object := func(name, computed any) tftypes.Value {
		return tftypes.NewValue(typ, map[string]tftypes.Value{"name": stringValue(name), "computed": stringValue(computed)})
	}
	for _, tc := range []struct {
		name        string
		plan, state tftypes.Value
		want        bool
	}{
		{"unchanged with unknown computed", object("same", tftypes.UnknownValue), object("same", "server value"), true},
		{"changed configured value", object("new", tftypes.UnknownValue), object("old", "server value"), false},
		{"explicit removal", object("same", nil), object("same", "server value"), false},
		{"computed null", object("same", tftypes.UnknownValue), object("same", nil), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := knownPlanMatchesState(tc.plan, tc.state); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
	listType := tftypes.List{ElementType: tftypes.String}
	original := tftypes.NewValue(listType, []tftypes.Value{stringValue("a"), stringValue("b")})
	reordered := tftypes.NewValue(listType, []tftypes.Value{stringValue("b"), stringValue("a")})
	if knownPlanMatchesState(reordered, original) {
		t.Fatal("must preserve list reordering as a change")
	}
}

func TestPreserveStepKinds(t *testing.T) {
	actual := escalationChainModelFromAPI(map[string]any{"steps": []any{map[string]any{"kind": "WAIT", "delay_minutes": float64(2)}}})
	prior := escalationChainModelFromAPI(map[string]any{"steps": []any{map[string]any{"kind": "wait", "delay_minutes": float64(2)}}})
	got := preserveStepKinds(actual.Steps, prior.Steps)
	if !got.Equal(prior.Steps) {
		t.Fatal("API canonicalization must preserve equivalent configured kind")
	}
	changed := escalationChainModelFromAPI(map[string]any{"steps": []any{map[string]any{"kind": "RESOLVE"}}})
	if preserveStepKinds(changed.Steps, prior.Steps).Equal(prior.Steps) {
		t.Fatal("must not conceal a changed step kind")
	}
}
