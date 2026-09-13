package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &IntegrationResource{}
var _ resource.ResourceWithImportState = &IntegrationResource{}

type IntegrationResource struct{ client *client }

func NewIntegrationResource() resource.Resource { return &IntegrationResource{} }

func (r *IntegrationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration"
}

type integrationModel struct {
	ID                 types.String `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	Key                types.String `tfsdk:"key"`
	Type               types.String `tfsdk:"type"`
	SourceType         types.String `tfsdk:"source_type"`
	GroupBy            types.List   `tfsdk:"group_by"`
	Routes             types.List   `tfsdk:"routes"`
	NotificationPolicy types.Object `tfsdk:"notification_policy"`
	Templates          types.Map    `tfsdk:"templates"`
	WebhookSecret      types.String `tfsdk:"webhook_secret"`
	TeamID             types.String `tfsdk:"team_id"`
	KafkaTopic         types.String `tfsdk:"kafka_topic"`
	Pipeline           types.String `tfsdk:"pipeline"`
	ProvisionedBy      types.String `tfsdk:"provisioned_by"`
	Heartbeat          types.Object `tfsdk:"heartbeat"`
	LegacyPool         types.Object `tfsdk:"legacy_pool"`
	CreatedAt          types.String `tfsdk:"created_at"`
	UpdatedAt          types.String `tfsdk:"updated_at"`
}

// routeAttrTypes reflects the real engine route structure.
var routeAttrTypes = map[string]attr.Type{
	"id":                  types.StringType,
	"name":                types.StringType,
	"match_type":          types.StringType, // all | labels | regex
	"is_default":          types.BoolType,
	"labels":              types.MapType{ElemType: types.StringType},
	"pattern":             types.StringType, // regex pattern
	"escalation_chain_id": types.StringType,
}

var notificationPolicyAttrTypes = map[string]attr.Type{
	"channels":               types.ListType{ElemType: types.StringType},
	"batch_timeout_seconds":  types.Int64Type,
	"batch_deadline_seconds": types.Int64Type,
	"epic_threshold_count":   types.Int64Type,
	"epic_threshold_seconds": types.Int64Type,
	"emergency_user_id":      types.StringType,
	"epic_user_id":           types.StringType,
}

var heartbeatAttrTypes = map[string]attr.Type{
	"interval_seconds": types.Int64Type,
	"grace_seconds":    types.Int64Type,
}

var legacyPoolAttrTypes = map[string]attr.Type{
	"name":              types.StringType,
	"description":       types.StringType,
	"emergency_user_id": types.StringType,
	"duty_user_id":      types.StringType,
	"emails":            types.ListType{ElemType: types.StringType},
}

func (r *IntegrationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a nxs-anomaly integration (alert ingestion endpoint).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{Required: true},
			"key": schema.StringAttribute{
				Computed:    true,
				Description: "Auto-generated integration key used in the webhook URL.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"type": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("webhook"),
				Description: "Integration type: webhook, alertmanager, pagerduty, victorops, grafana-alerting.",
				Validators:  []validator.String{stringvalidator.OneOf(validIntegrationTypes...)},
			},
			"source_type": schema.StringAttribute{Optional: true, Computed: true},
			"group_by": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "Alert fields used for grouping alerts into alert groups.",
			},
			"routes": schema.ListNestedAttribute{
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
				Optional:      true,
				Computed:      true,
				Description:   "Routing rules. Exactly one route must have is_default=true.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":   schema.StringAttribute{Optional: true, Computed: true},
						"name": schema.StringAttribute{Required: true},
						"match_type": schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Default:     stringdefault.StaticString("all"),
							Description: "Match strategy: all (catch-all), labels (match label map), regex (match payload regex).",
							Validators:  []validator.String{stringvalidator.OneOf(validRouteMatchTypes...)},
						},
						"is_default": schema.BoolAttribute{
							Optional:    true,
							Computed:    true,
							Default:     booldefault.StaticBool(false),
							Description: "Exactly one route per integration must be the default.",
						},
						"labels": schema.MapAttribute{
							Optional:    true,
							Computed:    true,
							ElementType: types.StringType,
							Description: "Label key=value pairs to match (match_type=labels).",
						},
						"pattern": schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Description: "Regex pattern applied to the JSON payload (match_type=regex).",
						},
						"escalation_chain_id": schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Description: "ID of the escalation chain to trigger.",
						},
					},
				},
			},
			"notification_policy": schema.SingleNestedAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Controls notification batching and escalation behaviour.",
				Attributes: map[string]schema.Attribute{
					"channels": schema.ListAttribute{
						Optional:    true,
						Computed:    true,
						ElementType: types.StringType,
						Description: "Notification channels: webhook, telegram, email, sms, phone.",
					},
					"batch_timeout_seconds":  schema.Int64Attribute{Optional: true, Computed: true},
					"batch_deadline_seconds": schema.Int64Attribute{Optional: true, Computed: true},
					"epic_threshold_count":   schema.Int64Attribute{Optional: true, Computed: true},
					"epic_threshold_seconds": schema.Int64Attribute{Optional: true, Computed: true},
					"emergency_user_id":      schema.StringAttribute{Optional: true, Computed: true},
					"epic_user_id":           schema.StringAttribute{Optional: true, Computed: true},
				},
			},
			"templates": schema.MapAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "Go template strings keyed by channel name (telegram, email, webhook, sms, phone; \"default\" covers the rest) used for notification rendering. Variables: title, severity, reason, group_id, status, user_name, user_username, labels (all of the alert's labels as \"k=v, k=v\"), and label_<name> for each one — so {{ .label_pod }} and {{ .label_namespace }} name the pod and namespace. Characters a template identifier cannot hold become underscores, so kubernetes.io/name is label_kubernetes_io_name. E.g. \"*[{{ .severity }}]* {{ .title }} ({{ .label_namespace }})\"; CamelCase aliases such as {{ .Severity }} also resolve. Removing the attribute keeps the value already stored; set templates = {} to clear them and fall back to the built-in format.",
			},
			"pipeline": schema.StringAttribute{
				Optional:   true,
				Computed:   true,
				Validators: []validator.String{jsonArrayValidator{}},
				Description: "Ingest pipeline as a JSON array of stages, each one action " +
					"(extract, set, rename, remove, gsub, truncate, drop) with an optional \"if\". " +
					"Runs AFTER route selection, so it shapes deduplication, the stored alert and the " +
					"notification text but never moves which escalation chain pages — the route " +
					"follows from the alert as the source sent it. Pass it with jsonencode(); the API " +
					"rejects a pipeline that does not compile, so a bad rule fails the apply rather " +
					"than the next incident. See docs/CONFIGURATION.md in nxs-anomaly.",
			},
			"provisioned_by": schema.StringAttribute{
				Computed: true,
				Description: "Set to \"terraform\" for objects this provider created. The API then " +
					"refuses edits from anyone else and the web UI disables its own controls, so an " +
					"out-of-band change cannot be silently undone by the next apply.",
			},
			"webhook_secret": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Sensitive:   true,
				Description: "HMAC secret for webhook signature validation (X-Hub-Signature-256).",
			},
			"team_id": schema.StringAttribute{
				Optional: true, Computed: true,
				Description: "ID of the team that owns this integration.",
			},
			"kafka_topic": schema.StringAttribute{
				Optional: true, Computed: true,
				Description: "Kafka topic used for integration outbox events.",
			},
			"heartbeat": schema.SingleNestedAttribute{
				Optional: true, Computed: true,
				Description: "Silence detection: raise a SourceSilent alert when this source stops " +
					"sending. Opt-in — with interval_seconds unset (0) the integration is never " +
					"reported for being quiet.",
				Attributes: map[string]schema.Attribute{
					"interval_seconds": schema.Int64Attribute{
						Optional: true, Computed: true,
						Description: "How long the source may stay silent before that is news. " +
							"Values below 60 are raised to 60 by the API; 0 disables the check.",
					},
					"grace_seconds": schema.Int64Attribute{
						Optional: true, Computed: true,
						Description: "Extra slack for a late tick. Defaults to a third of " +
							"interval_seconds when the check is enabled and this is left at 0.",
					},
				},
			},
			"legacy_pool": schema.SingleNestedAttribute{
				Optional: true, Computed: true,
				Description: "Legacy duty pool used by legacy ingestion payloads.",
				Attributes: map[string]schema.Attribute{
					"name":              schema.StringAttribute{Optional: true, Computed: true},
					"description":       schema.StringAttribute{Optional: true, Computed: true},
					"emergency_user_id": schema.StringAttribute{Optional: true, Computed: true},
					"duty_user_id":      schema.StringAttribute{Optional: true, Computed: true},
					"emails":            schema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType},
				},
			},
			"created_at": schema.StringAttribute{Computed: true},
			"updated_at": schema.StringAttribute{Computed: true},
		},
	}
}

func (r *IntegrationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client)
	if !ok {
		resp.Diagnostics.AddError("unexpected provider data type", fmt.Sprintf("got %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *IntegrationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan integrationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := integrationBody(ctx, plan)
	result, err := r.client.post(ctx, "/api/v1/integrations", body)
	if err != nil {
		resp.Diagnostics.AddError("create integration failed", err.Error())
		return
	}
	model := integrationModelFromAPI(result)
	if !plan.WebhookSecret.IsNull() && !plan.WebhookSecret.IsUnknown() {
		model.WebhookSecret = plan.WebhookSecret
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *IntegrationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state integrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := r.client.get(ctx, "/api/v1/integrations/"+state.ID.ValueString())
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read integration failed", err.Error())
		return
	}
	model := integrationModelFromAPI(result)
	if !state.WebhookSecret.IsNull() {
		model.WebhookSecret = state.WebhookSecret
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *IntegrationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan integrationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := integrationBody(ctx, plan)
	result, err := r.client.put(ctx, "/api/v1/integrations/"+plan.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("update integration failed", err.Error())
		return
	}
	model := integrationModelFromAPI(result)
	if !plan.WebhookSecret.IsNull() && !plan.WebhookSecret.IsUnknown() {
		model.WebhookSecret = plan.WebhookSecret
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *IntegrationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state integrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.delete(ctx, "/api/v1/integrations/"+state.ID.ValueString()); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete integration failed", err.Error())
	}
}

func (r *IntegrationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// integrationBody builds the API payload from a plan model.
func integrationBody(ctx context.Context, plan integrationModel) map[string]any {
	body := map[string]any{
		"name":     plan.Name.ValueString(),
		"type":     plan.Type.ValueString(),
		"group_by": stringListToAny(ctx, plan.GroupBy),
		"routes":   routesFromPlan(ctx, plan.Routes),
	}
	if !plan.SourceType.IsNull() && !plan.SourceType.IsUnknown() {
		body["source_type"] = plan.SourceType.ValueString()
	}
	if !plan.WebhookSecret.IsNull() && !plan.WebhookSecret.IsUnknown() {
		body["webhook_secret"] = plan.WebhookSecret.ValueString()
	}
	if !plan.NotificationPolicy.IsNull() && !plan.NotificationPolicy.IsUnknown() {
		body["notification_policy"] = notificationPolicyFromPlan(ctx, plan.NotificationPolicy)
	}
	if !plan.Pipeline.IsNull() && !plan.Pipeline.IsUnknown() {
		// Shape already checked at plan time by jsonArrayValidator, so a decode
		// failure here is impossible rather than merely unlikely.
		body["pipeline"], _ = decodePipeline(plan.Pipeline.ValueString())
	}
	if !plan.Templates.IsNull() && !plan.Templates.IsUnknown() {
		body["templates"] = templatesFromPlan(ctx, plan.Templates)
	}
	if !plan.TeamID.IsNull() && !plan.TeamID.IsUnknown() {
		body["team_id"] = plan.TeamID.ValueString()
	}
	if !plan.KafkaTopic.IsNull() && !plan.KafkaTopic.IsUnknown() {
		body["kafka_topic"] = plan.KafkaTopic.ValueString()
	}
	if !plan.Heartbeat.IsNull() && !plan.Heartbeat.IsUnknown() {
		body["heartbeat"] = heartbeatFromPlan(plan.Heartbeat)
	}
	if !plan.LegacyPool.IsNull() && !plan.LegacyPool.IsUnknown() {
		body["legacy_pool"] = legacyPoolFromPlan(ctx, plan.LegacyPool)
	}
	return body
}

func heartbeatFromPlan(obj types.Object) map[string]any {
	a := obj.Attributes()
	return map[string]any{
		"interval_seconds": attrInt64(a, "interval_seconds", 0),
		"grace_seconds":    attrInt64(a, "grace_seconds", 0),
	}
}

func legacyPoolFromPlan(ctx context.Context, obj types.Object) map[string]any {
	a := obj.Attributes()
	pool := map[string]any{
		"name": attrStr(a, "name"), "description": attrStr(a, "description"),
		"emergency_user_id": attrStr(a, "emergency_user_id"),
		"duty_user_id":      attrStr(a, "duty_user_id"),
	}
	if emails, ok := a["emails"].(types.List); ok && !emails.IsNull() && !emails.IsUnknown() {
		pool["emails"] = stringListToAny(ctx, emails)
	}
	return pool
}

func routesFromPlan(ctx context.Context, list types.List) []any {
	if list.IsNull() || list.IsUnknown() {
		return nil
	}
	var out []any
	for _, elem := range list.Elements() {
		obj, ok := elem.(types.Object)
		if !ok {
			continue
		}
		a := obj.Attributes()
		route := map[string]any{
			"name":       attrStr(a, "name"),
			"match_type": attrStr(a, "match_type"),
			"is_default": attrBool(a, "is_default", false),
		}
		if id := attrStr(a, "id"); id != "" {
			route["id"] = id
		}
		if v := attrStr(a, "escalation_chain_id"); v != "" {
			route["escalation_chain_id"] = v
		}
		if v := attrStr(a, "pattern"); v != "" {
			route["pattern"] = v
		}
		if lm, ok := a["labels"].(types.Map); ok && !lm.IsNull() && !lm.IsUnknown() {
			labels := make(map[string]any)
			for k, v := range lm.Elements() {
				if s, ok := v.(types.String); ok {
					labels[k] = s.ValueString()
				}
			}
			route["labels"] = labels
		}
		out = append(out, route)
	}
	return out
}

func notificationPolicyFromPlan(ctx context.Context, obj types.Object) map[string]any {
	if obj.IsNull() || obj.IsUnknown() {
		return nil
	}
	a := obj.Attributes()
	policy := map[string]any{
		"batch_timeout_seconds":  attrInt64(a, "batch_timeout_seconds", 0),
		"batch_deadline_seconds": attrInt64(a, "batch_deadline_seconds", 0),
		"epic_threshold_count":   attrInt64(a, "epic_threshold_count", 0),
		"epic_threshold_seconds": attrInt64(a, "epic_threshold_seconds", 0),
	}
	if v := attrStr(a, "emergency_user_id"); v != "" {
		policy["emergency_user_id"] = v
	}
	if v := attrStr(a, "epic_user_id"); v != "" {
		policy["epic_user_id"] = v
	}
	if ch, ok := a["channels"].(types.List); ok && !ch.IsNull() && !ch.IsUnknown() {
		policy["channels"] = stringListToAny(ctx, ch)
	}
	return policy
}

func templatesFromPlan(ctx context.Context, m types.Map) map[string]any {
	if m.IsNull() || m.IsUnknown() {
		return nil
	}
	out := make(map[string]any, len(m.Elements()))
	for k, v := range m.Elements() {
		if s, ok := v.(types.String); ok {
			out[k] = s.ValueString()
		}
	}
	return out
}

func integrationModelFromAPI(m map[string]any) integrationModel {
	groupBy := stringSliceFromMap(m, "group_by")

	rawRoutes := mapSliceFromMap(m, "routes")
	routeVals := make([]attr.Value, 0, len(rawRoutes))
	for _, r := range rawRoutes {
		labelsRaw, _ := r["labels"].(map[string]any)
		labelsMap := make(map[string]attr.Value, len(labelsRaw))
		for k, v := range labelsRaw {
			labelsMap[k] = types.StringValue(fmt.Sprintf("%v", v))
		}
		labelsObj, _ := types.MapValue(types.StringType, labelsMap)

		rObj, _ := types.ObjectValue(routeAttrTypes, map[string]attr.Value{
			"id":                  types.StringValue(strFromMap(r, "id")),
			"name":                types.StringValue(strFromMap(r, "name")),
			"match_type":          types.StringValue(strDefault(strFromMap(r, "match_type"), "all")),
			"is_default":          types.BoolValue(boolFromMap(r, "is_default", false)),
			"labels":              labelsObj,
			"pattern":             types.StringValue(strFromMap(r, "pattern")),
			"escalation_chain_id": types.StringValue(strFromMap(r, "escalation_chain_id")),
		})
		routeVals = append(routeVals, rObj)
	}
	routeList, _ := types.ListValue(types.ObjectType{AttrTypes: routeAttrTypes}, routeVals)

	// notification_policy
	var policyObj types.Object
	if pm, ok := m["notification_policy"].(map[string]any); ok && pm != nil {
		chList := stringSliceToList(stringSliceFromMap(pm, "channels"))
		policyObj, _ = types.ObjectValue(notificationPolicyAttrTypes, map[string]attr.Value{
			"channels":               chList,
			"batch_timeout_seconds":  types.Int64Value(int64FromMap(pm, "batch_timeout_seconds", 0)),
			"batch_deadline_seconds": types.Int64Value(int64FromMap(pm, "batch_deadline_seconds", 0)),
			"epic_threshold_count":   types.Int64Value(int64FromMap(pm, "epic_threshold_count", 0)),
			"epic_threshold_seconds": types.Int64Value(int64FromMap(pm, "epic_threshold_seconds", 0)),
			"emergency_user_id":      types.StringValue(strFromMap(pm, "emergency_user_id")),
			"epic_user_id":           types.StringValue(strFromMap(pm, "epic_user_id")),
		})
	} else {
		policyObj = types.ObjectNull(notificationPolicyAttrTypes)
	}

	// templates
	var templatesMap types.Map
	if tm, ok := m["templates"].(map[string]any); ok && tm != nil {
		tvals := make(map[string]attr.Value, len(tm))
		for k, v := range tm {
			tvals[k] = types.StringValue(fmt.Sprintf("%v", v))
		}
		templatesMap, _ = types.MapValue(types.StringType, tvals)
	} else {
		templatesMap = types.MapNull(types.StringType)
	}

	// The engine always writes a heartbeat block, zeroed when the check is off,
	// so an absent one means an older API rather than "disabled".
	heartbeat := types.ObjectNull(heartbeatAttrTypes)
	if hm, ok := m["heartbeat"].(map[string]any); ok && hm != nil {
		heartbeat, _ = types.ObjectValue(heartbeatAttrTypes, map[string]attr.Value{
			"interval_seconds": types.Int64Value(int64FromMap(hm, "interval_seconds", 0)),
			"grace_seconds":    types.Int64Value(int64FromMap(hm, "grace_seconds", 0)),
		})
	}

	legacyPool := types.ObjectNull(legacyPoolAttrTypes)
	if pm, ok := m["legacy_pool"].(map[string]any); ok && pm != nil {
		legacyPool, _ = types.ObjectValue(legacyPoolAttrTypes, map[string]attr.Value{
			"name":              types.StringValue(strFromMap(pm, "name")),
			"description":       types.StringValue(strFromMap(pm, "description")),
			"emergency_user_id": nullableString(strFromMap(pm, "emergency_user_id")),
			"duty_user_id":      nullableString(strFromMap(pm, "duty_user_id")),
			"emails":            stringSliceToList(stringSliceFromMap(pm, "emails")),
		})
	}

	return integrationModel{
		ID:                 types.StringValue(strFromMap(m, "id")),
		Name:               types.StringValue(strFromMap(m, "name")),
		Key:                types.StringValue(strFromMap(m, "key")),
		Type:               types.StringValue(strFromMap(m, "type")),
		SourceType:         types.StringValue(strFromMap(m, "source_type")),
		GroupBy:            stringSliceToList(groupBy),
		Routes:             routeList,
		NotificationPolicy: policyObj,
		Templates:          templatesMap,
		Pipeline:           encodePipeline(m["pipeline"]),
		ProvisionedBy:      nullableString(strFromMap(m, "provisioned_by")),
		WebhookSecret:      types.StringNull(),
		TeamID:             nullableString(strFromMap(m, "team_id")),
		KafkaTopic:         nullableString(strFromMap(m, "kafka_topic")),
		Heartbeat:          heartbeat,
		LegacyPool:         legacyPool,
		CreatedAt:          types.StringValue(strFromMap(m, "created_at")),
		UpdatedAt:          types.StringValue(strFromMap(m, "updated_at")),
	}
}

func nullableString(value string) types.String {
	if value == "" {
		return types.StringNull()
	}
	return types.StringValue(value)
}

func attrBool(a map[string]attr.Value, key string, def bool) bool {
	if v, ok := a[key].(types.Bool); ok && !v.IsNull() && !v.IsUnknown() {
		return v.ValueBool()
	}
	return def
}

func attrInt64(a map[string]attr.Value, key string, def int64) int64 {
	if v, ok := a[key].(types.Int64); ok && !v.IsNull() && !v.IsUnknown() {
		return v.ValueInt64()
	}
	return def
}

// decodePipeline turns the JSON the practitioner wrote into the structure the
// API expects. It only checks that the document is a JSON array — the engine
// validates the stages themselves and answers 400 with the reason, which is a
// better error than anything this provider could reproduce and one that cannot
// drift from the server's actual rules.
func decodePipeline(raw string) ([]any, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	var stages []any
	if err := json.Unmarshal([]byte(trimmed), &stages); err != nil {
		return nil, fmt.Errorf("pipeline must be a JSON array of stages, e.g. jsonencode([{ set = { team = \"payments\" } }]): %w", err)
	}
	return stages, nil
}

// encodePipeline renders what the API returned back into state.
//
// Marshalling the decoded value rather than echoing the practitioner's string
// keeps the two sides comparable: Go sorts object keys, so the same pipeline
// written with keys in a different order settles to one form instead of showing
// a permanent diff.
func encodePipeline(v any) types.String {
	if v == nil {
		return types.StringNull()
	}
	list, ok := v.([]any)
	if !ok || len(list) == 0 {
		return types.StringNull()
	}
	b, err := json.Marshal(list)
	if err != nil {
		return types.StringNull()
	}
	return types.StringValue(string(b))
}

// jsonArrayValidator rejects a pipeline that is not a JSON array before the plan
// is applied.
//
// The engine validates the stages themselves and its message names the offending
// stage, so this deliberately checks only the outer shape: duplicating the rules
// here would produce a second source of truth that drifts, and the one thing
// worth catching early is the mistake a practitioner actually makes — passing an
// object, or a string that was never jsonencode()d.
type jsonArrayValidator struct{}

func (jsonArrayValidator) Description(context.Context) string {
	return "must be a JSON array of pipeline stages"
}

func (v jsonArrayValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (jsonArrayValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if _, err := decodePipeline(req.ConfigValue.ValueString()); err != nil {
		resp.Diagnostics.AddAttributeError(req.Path, "invalid pipeline", err.Error())
	}
}
