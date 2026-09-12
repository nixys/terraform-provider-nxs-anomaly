package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Optional computed nested lists can trigger framework unknown marking before
// their UseStateForUnknown modifier restores the list. If every known planned
// value still matches refreshed state, restore the remaining computed values
// too. Never do this while configuration itself contains unresolved values.
func preserveUnchangedPlan(req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() || req.State.Raw.IsNull() || !req.Config.Raw.IsFullyKnown() {
		return
	}
	if knownPlanMatchesState(req.Plan.Raw, req.State.Raw) {
		resp.Plan.Raw = req.State.Raw
	}
}

func knownPlanMatchesState(plan, state tftypes.Value) bool {
	if !plan.IsKnown() {
		return true
	}
	if plan.IsNull() || state.IsNull() || !state.IsKnown() {
		return plan.Equal(state)
	}
	switch plan.Type().(type) {
	case tftypes.Object, tftypes.Map:
		var planned, previous map[string]tftypes.Value
		if plan.As(&planned) != nil || state.As(&previous) != nil || len(planned) != len(previous) {
			return false
		}
		for key, value := range planned {
			old, ok := previous[key]
			if !ok || !knownPlanMatchesState(value, old) {
				return false
			}
		}
		return true
	case tftypes.List, tftypes.Tuple:
		var planned, previous []tftypes.Value
		if plan.As(&planned) != nil || state.As(&previous) != nil || len(planned) != len(previous) {
			return false
		}
		for i, value := range planned {
			if !knownPlanMatchesState(value, previous[i]) {
				return false
			}
		}
		return true
	default:
		return plan.Equal(state)
	}
}

var _ resource.ResourceWithModifyPlan = &UserResource{}
var _ resource.ResourceWithModifyPlan = &IntegrationResource{}
var _ resource.ResourceWithModifyPlan = &EscalationChainResource{}

func (r *UserResource) ModifyPlan(_ context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	preserveUnchangedPlan(req, resp)
}
func (r *IntegrationResource) ModifyPlan(_ context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	preserveUnchangedPlan(req, resp)
}
func (r *EscalationChainResource) ModifyPlan(_ context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	preserveUnchangedPlan(req, resp)
}
