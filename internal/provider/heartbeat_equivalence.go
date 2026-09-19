package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// keepEquivalentHeartbeat restores the heartbeat values as the practitioner
// wrote them when the API only normalised them.
//
// The API raises interval_seconds below 60 to 60 and turns grace_seconds = 0
// into a third of the interval. A configuration that spells either the way the
// API does not — `grace_seconds = 0` is what the module example passes by
// default — came back different from the plan, and Terraform failed the apply
// with "Provider produced inconsistent result", tainted the integration and
// replaced it on the next apply with a new routing key, every time. A value the
// API derives from the configured one is not a change; anything else is real
// drift and is reported.
func keepEquivalentHeartbeat(current, prior types.Object) types.Object {
	if current.IsNull() || current.IsUnknown() || prior.IsNull() || prior.IsUnknown() {
		return current
	}
	cur, pri := current.Attributes(), prior.Attributes()
	ci, okCI := cur["interval_seconds"].(types.Int64)
	cg, okCG := cur["grace_seconds"].(types.Int64)
	pi, okPI := pri["interval_seconds"].(types.Int64)
	pg, okPG := pri["grace_seconds"].(types.Int64)
	if !okCI || !okCG || !okPI || !okPG {
		return current
	}
	next := map[string]attr.Value{"interval_seconds": ci, "grace_seconds": cg}
	if known(pi) && normalisedInterval(pi.ValueInt64()) == ci.ValueInt64() {
		next["interval_seconds"] = pi
	}
	if known(pg) && normalisedGrace(ci.ValueInt64(), pg.ValueInt64()) == cg.ValueInt64() {
		next["grace_seconds"] = pg
	}
	obj, diags := types.ObjectValue(heartbeatAttrTypes, next)
	if diags.HasError() {
		return current
	}
	return obj
}

func known(v types.Int64) bool { return !v.IsNull() && !v.IsUnknown() }

// normalisedInterval and normalisedGrace mirror the API's sanitizeHeartbeat.
func normalisedInterval(v int64) int64 {
	if v < 0 {
		return 0
	}
	if v > 0 && v < 60 {
		return 60
	}
	return v
}

func normalisedGrace(interval, grace int64) int64 {
	if grace < 0 {
		grace = 0
	}
	if interval > 0 && grace == 0 {
		return interval / 3
	}
	return grace
}
