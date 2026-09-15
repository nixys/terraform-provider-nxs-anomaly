package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// keepInstant returns prior when it names the same instant as current. The API
// answers in its own zone — UTC, or the schedule's timezone — so a time written
// with another offset comes back as a different string for the same moment, and
// Terraform reads that as an inconsistent apply result or as drift.
func keepInstant(prior, current types.String) types.String {
	if sameInstant(prior.ValueString(), current.ValueString()) {
		return prior
	}
	return current
}

// keepObjectInstants applies keepInstant to the named string attributes of an
// object, leaving every other attribute as the API returned it.
func keepObjectInstants(current, prior types.Object, attrTypes map[string]attr.Type, keys ...string) types.Object {
	if current.IsNull() || current.IsUnknown() || prior.IsNull() || prior.IsUnknown() {
		return current
	}
	next := make(map[string]attr.Value, len(current.Attributes()))
	for k, v := range current.Attributes() {
		next[k] = v
	}
	priorAttrs := prior.Attributes()
	for _, k := range keys {
		c, okCurrent := next[k].(types.String)
		p, okPrior := priorAttrs[k].(types.String)
		if okCurrent && okPrior {
			next[k] = keepInstant(p, c)
		}
	}
	obj, diags := types.ObjectValue(attrTypes, next)
	if diags.HasError() {
		return current
	}
	return obj
}
