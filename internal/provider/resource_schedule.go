package provider

import (
	"context"
	"fmt"
	"time"
	_ "time/tzdata"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &ScheduleResource{}
var _ resource.ResourceWithImportState = &ScheduleResource{}

type ScheduleResource struct{ client *client }

func NewScheduleResource() resource.Resource { return &ScheduleResource{} }

func (r *ScheduleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_schedule"
}

type scheduleModel struct {
	ID                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	Timezone            types.String `tfsdk:"timezone"`
	TeamID              types.String `tfsdk:"team_id"`
	Enabled             types.Bool   `tfsdk:"enabled"`
	NotifyOnShiftChange types.Bool   `tfsdk:"notify_on_shift_change"`
	Rotation            types.Object `tfsdk:"rotation"`
	Shifts              types.List   `tfsdk:"shifts"`
	CreatedAt           types.String `tfsdk:"created_at"`
	ProvisionedBy       types.String `tfsdk:"provisioned_by"`
	UpdatedAt           types.String `tfsdk:"updated_at"`
}

var shiftAttrTypes = map[string]attr.Type{
	"id":         types.StringType,
	"user_id":    types.StringType,
	"start_at":   types.StringType,
	"end_at":     types.StringType,
	"recurrence": types.StringType,
}

var rotationRestrictionAttrTypes = map[string]attr.Type{
	"start": types.StringType,
	"end":   types.StringType,
	"days":  types.ListType{ElemType: types.StringType},
}

var rotationAttrTypes = map[string]attr.Type{
	"enabled":          types.BoolType,
	"start_at":         types.StringType,
	"handoff_interval": types.Int64Type,
	"handoff_unit":     types.StringType,
	"participant_ids":  types.ListType{ElemType: types.StringType},
	"restriction":      types.ObjectType{AttrTypes: rotationRestrictionAttrTypes},
}

func (r *ScheduleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a nxs-anomaly on-call schedule.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{Required: true},
			"timezone": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("UTC"),
			},
			"team_id": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "ID of the team that owns this schedule.",
			},
			"enabled": schema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(true),
				Description: "Whether this schedule participates in on-call resolution.",
			},
			"notify_on_shift_change": schema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				Description: "Notify participants when the active on-call shift changes.",
			},
			"rotation": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{
						Optional: true, Computed: true, Default: booldefault.StaticBool(true),
					},
					"start_at": schema.StringAttribute{Required: true},
					"handoff_interval": schema.Int64Attribute{
						Optional: true, Computed: true, Default: int64default.StaticInt64(1),
					},
					"handoff_unit": schema.StringAttribute{
						Optional: true, Computed: true, Default: stringdefault.StaticString("weeks"),
						Validators: []validator.String{stringvalidator.OneOf("hours", "days", "weeks")},
					},
					"participant_ids": schema.ListAttribute{Required: true, ElementType: types.StringType},
					"restriction": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"start": schema.StringAttribute{Required: true},
							"end":   schema.StringAttribute{Required: true},
							"days":  schema.ListAttribute{Optional: true, ElementType: types.StringType},
						},
					},
				},
			},
			"shifts": schema.ListNestedAttribute{
				Optional:    true,
				Computed:    true,
				Description: "List of on-call shifts.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Description: "Shift ID (computed if not provided).",
						},
						"user_id": schema.StringAttribute{
							Required:    true,
							Description: "ID of the user on duty during this shift.",
						},
						"start_at": schema.StringAttribute{
							Required:    true,
							Description: "Shift start time (RFC3339).",
						},
						"end_at": schema.StringAttribute{
							Required:    true,
							Description: "Shift end time (RFC3339).",
						},
						"recurrence": schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Default:     stringdefault.StaticString("none"),
							Description: "Recurrence pattern: none, daily, weekly.",
							Validators:  []validator.String{stringvalidator.OneOf(validShiftRecurrences...)},
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

func (r *ScheduleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ScheduleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan scheduleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"name":                   plan.Name.ValueString(),
		"timezone":               plan.Timezone.ValueString(),
		"enabled":                plan.Enabled.ValueBool(),
		"notify_on_shift_change": plan.NotifyOnShiftChange.ValueBool(),
		"shifts":                 shiftsFromPlan(ctx, plan.Shifts),
	}
	if !plan.Rotation.IsNull() && !plan.Rotation.IsUnknown() {
		body["rotation"] = rotationFromPlan(ctx, plan.Rotation)
	}
	if !plan.TeamID.IsNull() && !plan.TeamID.IsUnknown() && plan.TeamID.ValueString() != "" {
		body["team_id"] = plan.TeamID.ValueString()
	}

	result, err := r.client.post(ctx, "/api/v1/schedules", body)
	if err != nil {
		resp.Diagnostics.AddError("create schedule failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, scheduleModelFromAPI(result))...)
}

func (r *ScheduleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state scheduleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.get(ctx, "/api/v1/schedules/"+state.ID.ValueString())
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read schedule failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, scheduleModelFromAPI(result))...)
}

func (r *ScheduleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan scheduleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"name":                   plan.Name.ValueString(),
		"timezone":               plan.Timezone.ValueString(),
		"enabled":                plan.Enabled.ValueBool(),
		"notify_on_shift_change": plan.NotifyOnShiftChange.ValueBool(),
		"shifts":                 shiftsFromPlan(ctx, plan.Shifts),
	}
	if plan.Rotation.IsNull() {
		body["rotation"] = nil
	} else if !plan.Rotation.IsUnknown() {
		body["rotation"] = rotationFromPlan(ctx, plan.Rotation)
	}
	if !plan.TeamID.IsNull() && !plan.TeamID.IsUnknown() {
		body["team_id"] = plan.TeamID.ValueString()
	}

	result, err := r.client.put(ctx, "/api/v1/schedules/"+plan.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("update schedule failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, scheduleModelFromAPI(result))...)
}

func (r *ScheduleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state scheduleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.delete(ctx, "/api/v1/schedules/"+state.ID.ValueString()); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete schedule failed", err.Error())
	}
}

func (r *ScheduleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func shiftsFromPlan(ctx context.Context, list types.List) []any {
	if list.IsNull() || list.IsUnknown() {
		return nil
	}
	var out []any
	for _, elem := range list.Elements() {
		obj, ok := elem.(types.Object)
		if !ok {
			continue
		}
		attrs := obj.Attributes()
		shift := map[string]any{
			"user_id":    attrStr(attrs, "user_id"),
			"start_at":   attrStr(attrs, "start_at"),
			"end_at":     attrStr(attrs, "end_at"),
			"recurrence": attrStr(attrs, "recurrence"),
		}
		if id := attrStr(attrs, "id"); id != "" {
			shift["id"] = id
		}
		out = append(out, shift)
	}
	return out
}

func scheduleModelFromAPI(m map[string]any) scheduleModel {
	shifts := mapSliceFromMap(m, "shifts")
	timezone := strDefault(strFromMap(m, "timezone"), "UTC")
	var shiftVals []attr.Value
	for _, s := range shifts {
		obj, _ := types.ObjectValue(shiftAttrTypes, map[string]attr.Value{
			"id":         types.StringValue(strFromMap(s, "id")),
			"user_id":    types.StringValue(strFromMap(s, "user_id")),
			"start_at":   types.StringValue(formatTimeInLocation(strFromMap(s, "start_at"), timezone)),
			"end_at":     types.StringValue(formatTimeInLocation(strFromMap(s, "end_at"), timezone)),
			"recurrence": types.StringValue(strFromMap(s, "recurrence")),
		})
		shiftVals = append(shiftVals, obj)
	}
	shiftList, _ := types.ListValue(types.ObjectType{AttrTypes: shiftAttrTypes}, shiftVals)
	rotation := rotationModelFromAPI(m["rotation"], timezone)

	teamID := types.StringValue(strFromMap(m, "team_id"))
	if strFromMap(m, "team_id") == "" {
		teamID = types.StringNull()
	}

	return scheduleModel{
		ID:                  types.StringValue(strFromMap(m, "id")),
		Name:                types.StringValue(strFromMap(m, "name")),
		Timezone:            types.StringValue(timezone),
		TeamID:              teamID,
		Enabled:             types.BoolValue(boolFromMap(m, "enabled", true)),
		NotifyOnShiftChange: types.BoolValue(boolFromMap(m, "notify_on_shift_change", false)),
		Rotation:            rotation,
		Shifts:              shiftList,
		CreatedAt:           types.StringValue(strFromMap(m, "created_at")),
		ProvisionedBy:       nullableString(strFromMap(m, "provisioned_by")),
		UpdatedAt:           types.StringValue(strFromMap(m, "updated_at")),
	}
}

func rotationFromPlan(ctx context.Context, obj types.Object) map[string]any {
	if obj.IsNull() || obj.IsUnknown() {
		return nil
	}
	a := obj.Attributes()
	rotation := map[string]any{
		"enabled":          attrBool(a, "enabled", true),
		"start_at":         attrStr(a, "start_at"),
		"handoff_interval": attrInt64(a, "handoff_interval", 1),
		"handoff_unit":     attrStr(a, "handoff_unit"),
	}
	if ids, ok := a["participant_ids"].(types.List); ok {
		rotation["participant_ids"] = stringListToAny(ctx, ids)
	}
	if restriction, ok := a["restriction"].(types.Object); ok && !restriction.IsNull() && !restriction.IsUnknown() {
		ra := restriction.Attributes()
		r := map[string]any{"start": attrStr(ra, "start"), "end": attrStr(ra, "end")}
		if days, ok := ra["days"].(types.List); ok && !days.IsNull() && !days.IsUnknown() {
			r["days"] = stringListToAny(ctx, days)
		}
		rotation["restriction"] = r
	}
	return rotation
}

func rotationModelFromAPI(raw any, timezone string) types.Object {
	m, ok := raw.(map[string]any)
	if !ok || m == nil {
		return types.ObjectNull(rotationAttrTypes)
	}
	restriction := types.ObjectNull(rotationRestrictionAttrTypes)
	if rm, ok := m["restriction"].(map[string]any); ok && rm != nil {
		restriction, _ = types.ObjectValue(rotationRestrictionAttrTypes, map[string]attr.Value{
			"start": types.StringValue(strFromMap(rm, "start")),
			"end":   types.StringValue(strFromMap(rm, "end")),
			"days":  stringSliceToList(stringSliceFromMap(rm, "days")),
		})
	}
	obj, _ := types.ObjectValue(rotationAttrTypes, map[string]attr.Value{
		"enabled":          types.BoolValue(boolFromMap(m, "enabled", true)),
		"start_at":         types.StringValue(formatTimeInLocation(strFromMap(m, "start_at"), timezone)),
		"handoff_interval": types.Int64Value(int64FromMap(m, "handoff_interval", 1)),
		"handoff_unit":     types.StringValue(strDefault(strFromMap(m, "handoff_unit"), "weeks")),
		"participant_ids":  stringSliceToList(stringSliceFromMap(m, "participant_ids")),
		"restriction":      restriction,
	})
	return obj
}

func formatTimeInLocation(value, timezone string) string {
	if value == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return value
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return value
	}
	return t.In(loc).Format(time.RFC3339)
}

func attrStr(attrs map[string]attr.Value, key string) string {
	if v, ok := attrs[key]; ok {
		if s, ok := v.(types.String); ok {
			return s.ValueString()
		}
	}
	return ""
}
