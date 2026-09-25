package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &EscalationChainResource{}
var _ resource.ResourceWithImportState = &EscalationChainResource{}

type EscalationChainResource struct{ client *client }

func NewEscalationChainResource() resource.Resource { return &EscalationChainResource{} }

func (r *EscalationChainResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_escalation_chain"
}

type escalationChainModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	Steps         types.List   `tfsdk:"steps"`
	CreatedAt     types.String `tfsdk:"created_at"`
	ProvisionedBy types.String `tfsdk:"provisioned_by"`
	UpdatedAt     types.String `tfsdk:"updated_at"`
}

// stepAttrTypes covers all fields across all step kinds.
// Fields not applicable to a given kind are stored as zero/empty values.
var stepAttrTypes = map[string]attr.Type{
	"id":   types.StringType,
	"kind": types.StringType,
	// NOTIFY_USER
	"user_ids": types.ListType{ElemType: types.StringType},
	// NOTIFY_SCHEDULE
	"schedule_id":     types.StringType,
	"allow_uncovered": types.BoolType,
	// NOTIFY_TEAM / NOTIFY_DUTY_USERS
	"team_id":         types.StringType,
	"fallback_to_all": types.BoolType,
	// NOTIFY_EMERGENCY
	"user_id": types.StringType,
	// TRIGGER_WEBHOOK
	"webhook_url": types.StringType,
	"headers":     types.MapType{ElemType: types.StringType},
	// WAIT (field name matches engine: delay_minutes)
	"delay_minutes": types.Int64Type,
	// CREATE_ISSUE
	"tracker_type":     types.StringType,
	"url":              types.StringType,
	"token":            types.StringType,
	"token_env":        types.StringType,
	"project":          types.StringType,
	"subject_template": types.StringType,
	"body_template":    types.StringType,
	// REPEAT
	"from_position":    types.Int64Type,
	"max_repeat_count": types.Int64Type,
	"cooldown_minutes": types.Int64Type,
}

func (r *EscalationChainResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a nxs-anomaly escalation chain.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{Required: true},
			"steps": schema.ListNestedAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Ordered list of escalation steps.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{Optional: true, Computed: true},
						"kind": schema.StringAttribute{
							Required: true,
							Description: "Step kind: WAIT, NOTIFY_USER, NOTIFY_SCHEDULE, NOTIFY_TEAM, " +
								"NOTIFY_EMERGENCY, NOTIFY_DUTY_USERS, TRIGGER_WEBHOOK, CREATE_ISSUE, RESOLVE, REPEAT.",
							Validators: []validator.String{
								stringvalidator.OneOfCaseInsensitive(validStepKinds...),
							},
						},
						// NOTIFY_USER
						"user_ids": schema.ListAttribute{
							Optional:    true,
							Computed:    true,
							ElementType: types.StringType,
							Description: "User IDs to notify (NOTIFY_USER).",
						},
						// NOTIFY_SCHEDULE
						"schedule_id": schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Description: "Schedule ID (NOTIFY_SCHEDULE).",
						},
						"allow_uncovered": schema.BoolAttribute{
							Optional: true,
							Computed: true,
							Default:  booldefault.StaticBool(false),
							Description: "Accept a schedule with coverage gaps in the next 7 days " +
								"(NOTIFY_SCHEDULE). Without it the API refuses to attach such a " +
								"schedule, because the hole pages nobody and does so silently.",
						},
						// NOTIFY_TEAM / NOTIFY_DUTY_USERS
						"team_id": schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Description: "Team ID (NOTIFY_TEAM, NOTIFY_DUTY_USERS).",
						},
						"fallback_to_all": schema.BoolAttribute{
							Optional:    true,
							Computed:    true,
							Default:     booldefault.StaticBool(true),
							Description: "Notify all team members if no one is on duty (NOTIFY_DUTY_USERS).",
						},
						// NOTIFY_EMERGENCY
						"user_id": schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Description: "User ID for emergency notification (NOTIFY_EMERGENCY).",
						},
						// TRIGGER_WEBHOOK
						"webhook_url": schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Description: "Webhook URL (TRIGGER_WEBHOOK).",
						},
						"headers": schema.MapAttribute{
							Optional:    true,
							Sensitive:   true,
							ElementType: types.StringType,
							Description: "TRIGGER_WEBHOOK only. " + outboundHeadersDescription,
							Validators:  outboundHeadersValidators(),
						},
						// WAIT
						"delay_minutes": schema.Int64Attribute{
							Optional:    true,
							Computed:    true,
							Description: "Delay in minutes before the next step (WAIT).",
						},
						// CREATE_ISSUE
						"tracker_type": schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Description: "Issue tracker type, e.g. redmine (CREATE_ISSUE).",
						},
						"url": schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Description: "Issue tracker base URL (CREATE_ISSUE).",
						},
						"token": schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Sensitive:   true,
							Description: "API token for the issue tracker (CREATE_ISSUE).",
						},
						"token_env": schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Description: "Environment variable name holding the token (CREATE_ISSUE).",
						},
						"project": schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Description: "Issue tracker project identifier (CREATE_ISSUE).",
						},
						"subject_template": schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Description: "Go template for the issue subject (CREATE_ISSUE).",
						},
						"body_template": schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Description: "Go template for the issue body (CREATE_ISSUE).",
						},
						// REPEAT
						"from_position": schema.Int64Attribute{
							Optional:    true,
							Computed:    true,
							Description: "Step position (0-based) to jump back to (REPEAT).",
						},
						"max_repeat_count": schema.Int64Attribute{
							Optional:    true,
							Computed:    true,
							Description: "Maximum number of repetitions (REPEAT).",
						},
						"cooldown_minutes": schema.Int64Attribute{
							Optional:    true,
							Computed:    true,
							Description: "Minimum minutes between repeats (REPEAT).",
						},
					},
				},
			},
			"created_at": schema.StringAttribute{Computed: true},
			"provisioned_by": schema.StringAttribute{
				Computed: true,
				Description: "Set to \"terraform\" for objects this provider created. The API then " +
					"refuses edits from anyone else and the web UI disables its own controls, so an " +
					"out-of-band change cannot be silently undone by the next apply.",
			},
			"updated_at": schema.StringAttribute{Computed: true},
		},
	}
}

func (r *EscalationChainResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *EscalationChainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan escalationChainModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := map[string]any{
		"name":  plan.Name.ValueString(),
		"steps": stepsFromPlan(ctx, plan.Steps),
	}
	result, err := r.client.post(ctx, "/api/v1/escalation-chains", body)
	if err != nil {
		resp.Diagnostics.AddError("create escalation chain failed", err.Error())
		return
	}
	model := escalationChainModelFromAPI(result)
	model.Steps = preserveStepKinds(model.Steps, plan.Steps)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *EscalationChainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state escalationChainModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := r.client.get(ctx, "/api/v1/escalation-chains/"+state.ID.ValueString())
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read escalation chain failed", err.Error())
		return
	}
	model := escalationChainModelFromAPI(result)
	model.Steps = preserveStepKinds(model.Steps, state.Steps)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *EscalationChainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan escalationChainModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := map[string]any{
		"name":  plan.Name.ValueString(),
		"steps": stepsFromPlan(ctx, plan.Steps),
	}
	result, err := r.client.put(ctx, "/api/v1/escalation-chains/"+plan.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("update escalation chain failed", err.Error())
		return
	}
	model := escalationChainModelFromAPI(result)
	model.Steps = preserveStepKinds(model.Steps, plan.Steps)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *EscalationChainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state escalationChainModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.delete(ctx, "/api/v1/escalation-chains/"+state.ID.ValueString()); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete escalation chain failed", err.Error())
	}
}

func (r *EscalationChainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func stepsFromPlan(ctx context.Context, list types.List) []any {
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
		step := map[string]any{"kind": attrStr(a, "kind")}
		if id := attrStr(a, "id"); id != "" {
			step["id"] = id
		}
		setIfNonEmpty(step, a, "schedule_id")
		setIfNonEmpty(step, a, "team_id")
		setIfNonEmpty(step, a, "user_id")
		setIfNonEmpty(step, a, "webhook_url")
		if h := headersFromAttr(a["headers"]); h != nil {
			step["headers"] = h
		}
		setIfNonEmpty(step, a, "tracker_type")
		setIfNonEmpty(step, a, "url")
		setIfNonEmpty(step, a, "token")
		setIfNonEmpty(step, a, "token_env")
		setIfNonEmpty(step, a, "project")
		setIfNonEmpty(step, a, "subject_template")
		setIfNonEmpty(step, a, "body_template")
		setIfInt64Set(step, a, "delay_minutes")
		setIfInt64Set(step, a, "from_position")
		setIfInt64Set(step, a, "max_repeat_count")
		setIfInt64Set(step, a, "cooldown_minutes")
		if v, ok := a["fallback_to_all"].(types.Bool); ok && !v.IsNull() && !v.IsUnknown() {
			step["fallback_to_all"] = v.ValueBool()
		}
		if v, ok := a["allow_uncovered"].(types.Bool); ok && !v.IsNull() && !v.IsUnknown() {
			step["allow_uncovered"] = v.ValueBool()
		}
		if v, ok := a["user_ids"].(types.List); ok && !v.IsNull() && !v.IsUnknown() {
			step["user_ids"] = stringListToAny(ctx, v)
		}
		out = append(out, step)
	}
	return out
}

func setIfNonEmpty(dst map[string]any, a map[string]attr.Value, key string) {
	if v := attrStr(a, key); v != "" {
		dst[key] = v
	}
}

func setIfInt64Set(dst map[string]any, a map[string]attr.Value, key string) {
	if v, ok := a[key].(types.Int64); ok && !v.IsNull() && !v.IsUnknown() {
		dst[key] = v.ValueInt64()
	}
}

func escalationChainModelFromAPI(m map[string]any) escalationChainModel {
	rawSteps := mapSliceFromMap(m, "steps")
	var stepVals []attr.Value
	for _, s := range rawSteps {
		obj, _ := types.ObjectValue(stepAttrTypes, map[string]attr.Value{
			"id":               types.StringValue(strFromMap(s, "id")),
			"kind":             types.StringValue(strFromMap(s, "kind")),
			"user_ids":         stringSliceToList(stringSliceFromMap(s, "user_ids")),
			"schedule_id":      types.StringValue(strFromMap(s, "schedule_id")),
			"allow_uncovered":  types.BoolValue(boolFromMap(s, "allow_uncovered", false)),
			"team_id":          types.StringValue(strFromMap(s, "team_id")),
			"fallback_to_all":  types.BoolValue(boolFromMap(s, "fallback_to_all", true)),
			"user_id":          types.StringValue(strFromMap(s, "user_id")),
			"webhook_url":      types.StringValue(strFromMap(s, "webhook_url")),
			"headers":          headersToMap(s["headers"]),
			"delay_minutes":    types.Int64Value(int64FromMap(s, "delay_minutes", 0)),
			"tracker_type":     types.StringValue(strFromMap(s, "tracker_type")),
			"url":              types.StringValue(strFromMap(s, "url")),
			"token":            types.StringValue(strFromMap(s, "token")),
			"token_env":        types.StringValue(strFromMap(s, "token_env")),
			"project":          types.StringValue(strFromMap(s, "project")),
			"subject_template": types.StringValue(strFromMap(s, "subject_template")),
			"body_template":    types.StringValue(strFromMap(s, "body_template")),
			"from_position":    types.Int64Value(int64FromMap(s, "from_position", 0)),
			"max_repeat_count": types.Int64Value(int64FromMap(s, "max_repeat_count", 0)),
			"cooldown_minutes": types.Int64Value(int64FromMap(s, "cooldown_minutes", 0)),
		})
		stepVals = append(stepVals, obj)
	}
	stepList, _ := types.ListValue(types.ObjectType{AttrTypes: stepAttrTypes}, stepVals)
	return escalationChainModel{
		ID:            types.StringValue(strFromMap(m, "id")),
		Name:          types.StringValue(strFromMap(m, "name")),
		Steps:         stepList,
		CreatedAt:     types.StringValue(strFromMap(m, "created_at")),
		ProvisionedBy: nullableString(strFromMap(m, "provisioned_by")),
		UpdatedAt:     types.StringValue(strFromMap(m, "updated_at")),
	}
}

// preserveStepKinds keeps equivalent user spelling while retaining API values for all other fields.
func preserveStepKinds(actual, prior types.List) types.List {
	if actual.IsNull() || actual.IsUnknown() || prior.IsNull() || prior.IsUnknown() {
		return actual
	}
	values, previous := actual.Elements(), prior.Elements()
	for i, value := range values {
		if i >= len(previous) {
			break
		}
		current, ok := value.(types.Object)
		old, oldOK := previous[i].(types.Object)
		if !ok || !oldOK || current.IsNull() || old.IsNull() || old.IsUnknown() {
			continue
		}
		attrs, oldAttrs := current.Attributes(), old.Attributes()
		kind, kindOK := attrs["kind"].(types.String)
		oldKind, oldKindOK := oldAttrs["kind"].(types.String)
		if kindOK && oldKindOK && !oldKind.IsUnknown() && !oldKind.IsNull() && strings.EqualFold(kind.ValueString(), oldKind.ValueString()) {
			attrs["kind"] = oldKind
			updated, diags := types.ObjectValue(stepAttrTypes, attrs)
			if !diags.HasError() {
				values[i] = updated
			}
		}
	}
	result, diags := types.ListValue(actual.ElementType(context.Background()), values)
	if diags.HasError() {
		return actual
	}
	return result
}
