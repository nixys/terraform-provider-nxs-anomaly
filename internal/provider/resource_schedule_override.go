package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &ScheduleOverrideResource{}
var _ resource.ResourceWithImportState = &ScheduleOverrideResource{}

type ScheduleOverrideResource struct{ client *client }

func NewScheduleOverrideResource() resource.Resource { return &ScheduleOverrideResource{} }

func (r *ScheduleOverrideResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_schedule_override"
}

type scheduleOverrideModel struct {
	ID         types.String `tfsdk:"id"`
	ScheduleID types.String `tfsdk:"schedule_id"`
	UserID     types.String `tfsdk:"user_id"`
	StartAt    types.String `tfsdk:"start_at"`
	Until      types.String `tfsdk:"until"`
	Reason     types.String `tfsdk:"reason"`
	CreatedAt  types.String `tfsdk:"created_at"`
	UpdatedAt  types.String `tfsdk:"updated_at"`
}

func (r *ScheduleOverrideResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a temporary override of a nxs-anomaly on-call schedule.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			}},
			"schedule_id": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			}},
			"user_id":  schema.StringAttribute{Required: true},
			"start_at": schema.StringAttribute{Required: true, Description: "Override start time (RFC3339)."},
			"until":    schema.StringAttribute{Required: true, Description: "Override end time (RFC3339)."},
			"reason": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString(""),
			},
			"created_at": schema.StringAttribute{Computed: true},
			"updated_at": schema.StringAttribute{Computed: true},
		},
	}
}

func (r *ScheduleOverrideResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ScheduleOverrideResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan scheduleOverrideModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := r.client.post(ctx, scheduleOverridesPath(plan.ScheduleID.ValueString()), scheduleOverrideBody(plan))
	if err != nil {
		resp.Diagnostics.AddError("create schedule override failed", err.Error())
		return
	}
	override, schedule := overrideResponse(result)
	resp.Diagnostics.Append(resp.State.Set(ctx, scheduleOverrideModelFromAPI(override, schedule, plan.ScheduleID.ValueString()))...)
}

func (r *ScheduleOverrideResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state scheduleOverrideModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	schedule, err := r.client.get(ctx, "/api/v1/schedules/"+state.ScheduleID.ValueString())
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read schedule override failed", err.Error())
		return
	}
	for _, override := range mapSliceFromMap(schedule, "overrides") {
		if strFromMap(override, "id") == state.ID.ValueString() {
			resp.Diagnostics.Append(resp.State.Set(ctx, scheduleOverrideModelFromAPI(override, schedule, state.ScheduleID.ValueString()))...)
			return
		}
	}
	resp.State.RemoveResource(ctx)
}

func (r *ScheduleOverrideResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan scheduleOverrideModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := r.client.put(ctx, scheduleOverridesPath(plan.ScheduleID.ValueString())+"/"+plan.ID.ValueString(), scheduleOverrideBody(plan))
	if err != nil {
		resp.Diagnostics.AddError("update schedule override failed", err.Error())
		return
	}
	override, schedule := overrideResponse(result)
	resp.Diagnostics.Append(resp.State.Set(ctx, scheduleOverrideModelFromAPI(override, schedule, plan.ScheduleID.ValueString()))...)
}

func (r *ScheduleOverrideResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state scheduleOverrideModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.delete(ctx, scheduleOverridesPath(state.ScheduleID.ValueString())+"/"+state.ID.ValueString())
	if err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete schedule override failed", err.Error())
	}
}

func (r *ScheduleOverrideResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("invalid import id", "expected <schedule_id>/<override_id>")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("schedule_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

func scheduleOverridesPath(scheduleID string) string {
	return "/api/v1/schedules/" + scheduleID + "/overrides"
}

func scheduleOverrideBody(model scheduleOverrideModel) map[string]any {
	return map[string]any{
		"user_id": model.UserID.ValueString(), "start_at": model.StartAt.ValueString(),
		"until": model.Until.ValueString(), "reason": model.Reason.ValueString(),
	}
}

func overrideResponse(result map[string]any) (map[string]any, map[string]any) {
	override, _ := result["override"].(map[string]any)
	schedule, _ := result["schedule"].(map[string]any)
	return override, schedule
}

func scheduleOverrideModelFromAPI(override, schedule map[string]any, scheduleID string) scheduleOverrideModel {
	timezone := strDefault(strFromMap(schedule, "timezone"), "UTC")
	return scheduleOverrideModel{
		ID:         types.StringValue(strFromMap(override, "id")),
		ScheduleID: types.StringValue(scheduleID),
		UserID:     types.StringValue(strFromMap(override, "user_id")),
		StartAt:    types.StringValue(formatTimeInLocation(strFromMap(override, "start_at"), timezone)),
		Until:      types.StringValue(formatTimeInLocation(strFromMap(override, "until"), timezone)),
		Reason:     types.StringValue(strFromMap(override, "reason")),
		CreatedAt:  types.StringValue(strFromMap(override, "created_at")),
		UpdatedAt:  types.StringValue(strFromMap(override, "updated_at")),
	}
}
