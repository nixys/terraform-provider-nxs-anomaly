package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Outbound headers (nxs-anomaly 1.7.0): sent with every post of a
// TRIGGER_WEBHOOK step or a ChatOps channel, so a gateway can authenticate the
// caller without its key in the URL. Values are secrets like webhook_url: an
// "env:VARIABLE" reference is resolved by nxs-anomaly at send time.

const outboundHeadersDescription = "Headers sent with every outbound post, retries included — typically " +
	"`Authorization` or `X-Api-Key` for a gateway that authenticates its callers. Values may be " +
	"`env:VARIABLE` references resolved by nxs-anomaly at send time (the production profile refuses " +
	"inline values). Names are written in canonical form (`X-Api-Key`, not `x-api-key`), as the API " +
	"stores them. `Host`, `Content-Length`, `Content-Type`, `Transfer-Encoding` and `Connection` cannot " +
	"be set. Requires nxs-anomaly 1.7.0 or newer."

// outboundHeadersValidators: at least one header (an empty map is stored as
// none and would read back as null), each name in canonical form.
func outboundHeadersValidators() []validator.Map {
	return []validator.Map{mapvalidator.SizeAtLeast(1), canonicalHeaderNames{}}
}

// canonicalHeaderNames requires names as net/http canonicalises them. The API
// stores them that way, so any other spelling would read back different from
// the configuration and fail the apply as an inconsistent result.
type canonicalHeaderNames struct{}

func (canonicalHeaderNames) Description(context.Context) string {
	return "header names must be in canonical form, e.g. X-Api-Key"
}

func (v canonicalHeaderNames) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (canonicalHeaderNames) ValidateMap(_ context.Context, req validator.MapRequest, resp *validator.MapResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	for name := range req.ConfigValue.Elements() {
		if canonical := http.CanonicalHeaderKey(name); canonical != name {
			resp.Diagnostics.AddAttributeError(req.Path, "Header name not in canonical form",
				fmt.Sprintf("Write %q as %q: nxs-anomaly stores header names in canonical form, and "+
					"another spelling would read back different from the configuration.", name, canonical))
		}
	}
}

// headersFromAttr is the request value for a configured headers map, or nil
// when it is not set.
func headersFromAttr(v attr.Value) map[string]any {
	m, ok := v.(types.Map)
	if !ok || m.IsNull() || m.IsUnknown() || len(m.Elements()) == 0 {
		return nil
	}
	out := make(map[string]any, len(m.Elements()))
	for k, e := range m.Elements() {
		if s, ok := e.(types.String); ok && !s.IsNull() && !s.IsUnknown() {
			out[k] = s.ValueString()
		}
	}
	return out
}

// headersToMap turns the API's headers object into state: null when there are
// none, which is what an unset optional attribute holds.
func headersToMap(raw any) types.Map {
	m, _ := raw.(map[string]any)
	if len(m) == 0 {
		return types.MapNull(types.StringType)
	}
	vals := make(map[string]attr.Value, len(m))
	for k, v := range m {
		vals[k] = types.StringValue(fmt.Sprintf("%v", v))
	}
	out, _ := types.MapValue(types.StringType, vals)
	return out
}
