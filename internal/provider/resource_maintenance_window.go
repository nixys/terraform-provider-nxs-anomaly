package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &MaintenanceWindowResource{}
var _ resource.ResourceWithImportState = &MaintenanceWindowResource{}

type MaintenanceWindowResource struct{ client *client }

func NewMaintenanceWindowResource() resource.Resource { return &MaintenanceWindowResource{} }

func (r *MaintenanceWindowResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_maintenance_window"
}

type maintenanceWindowModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Reason         types.String `tfsdk:"reason"`
	TeamID         types.String `tfsdk:"team_id"`
	IntegrationIDs types.List   `tfsdk:"integration_ids"`
	StartsAt       types.String `tfsdk:"starts_at"`
	EndsAt         types.String `tfsdk:"ends_at"`
	CreatedAt      types.String `tfsdk:"created_at"`
	ProvisionedBy  types.String `tfsdk:"provisioned_by"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

func (r *MaintenanceWindowResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a nxs-anomaly maintenance window. Alert groups opened by the named " +
			"integrations between starts_at and ends_at are recorded and silenced instead of paging anyone.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{Required: true},
			"reason": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Free-form explanation shown on silenced alert groups.",
			},
			"team_id": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "ID of the team that owns this window.",
			},
			"integration_ids": schema.ListAttribute{
				Required:    true,
				ElementType: types.StringType,
				Description: "Integrations covered by the window. Must name at least one: the API " +
					"rejects an empty list so that one typo cannot silence the whole deployment.",
			},
			"starts_at": schema.StringAttribute{
				Required:    true,
				Description: "Window start (RFC3339). The API returns it in UTC. After `terraform import`, a value written with another offset shows a one-time in-place update that changes nothing on the server.",
			},
			"ends_at": schema.StringAttribute{
				Required:    true,
				Description: "Window end (RFC3339). Must be after starts_at. The API returns it in UTC. After `terraform import`, a value written with another offset shows a one-time in-place update that changes nothing on the server.",
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

func (r *MaintenanceWindowResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *MaintenanceWindowResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan maintenanceWindowModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := r.client.post(ctx, "/api/v1/maintenance-windows", maintenanceWindowBody(ctx, plan))
	if err != nil {
		resp.Diagnostics.AddError("create maintenance window failed", err.Error())
		return
	}
	model := maintenanceWindowModelFromAPI(result)
	keepEquivalentBounds(&model, plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *MaintenanceWindowResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state maintenanceWindowModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := r.client.get(ctx, "/api/v1/maintenance-windows/"+state.ID.ValueString())
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read maintenance window failed", err.Error())
		return
	}
	model := maintenanceWindowModelFromAPI(result)
	keepEquivalentBounds(&model, state)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *MaintenanceWindowResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan maintenanceWindowModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := r.client.put(ctx, "/api/v1/maintenance-windows/"+plan.ID.ValueString(), maintenanceWindowBody(ctx, plan))
	if err != nil {
		resp.Diagnostics.AddError("update maintenance window failed", err.Error())
		return
	}
	model := maintenanceWindowModelFromAPI(result)
	keepEquivalentBounds(&model, plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *MaintenanceWindowResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state maintenanceWindowModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.delete(ctx, "/api/v1/maintenance-windows/"+state.ID.ValueString()); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete maintenance window failed", err.Error())
	}
}

func (r *MaintenanceWindowResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func maintenanceWindowBody(ctx context.Context, plan maintenanceWindowModel) map[string]any {
	body := map[string]any{
		"name":            plan.Name.ValueString(),
		"integration_ids": stringListToAny(ctx, plan.IntegrationIDs),
		"starts_at":       plan.StartsAt.ValueString(),
		"ends_at":         plan.EndsAt.ValueString(),
	}
	if !plan.Reason.IsNull() && !plan.Reason.IsUnknown() {
		body["reason"] = plan.Reason.ValueString()
	}
	if !plan.TeamID.IsNull() && !plan.TeamID.IsUnknown() {
		body["team_id"] = plan.TeamID.ValueString()
	}
	return body
}

// keepEquivalentBounds restores the timestamps as the practitioner wrote them.
//
// The engine stores bounds in UTC ("2026-06-01T06:00:00+00:00"), so a config
// written as "2026-06-01T09:00:00+03:00" would come back as a different string
// for the same instant — Terraform reads that as the provider producing a value
// it was not asked for. Same instant means no change; anything else is real
// drift and is reported.
func keepEquivalentBounds(model *maintenanceWindowModel, config maintenanceWindowModel) {
	if sameInstant(config.StartsAt.ValueString(), model.StartsAt.ValueString()) {
		model.StartsAt = config.StartsAt
	}
	if sameInstant(config.EndsAt.ValueString(), model.EndsAt.ValueString()) {
		model.EndsAt = config.EndsAt
	}
}

func sameInstant(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	ta, err := time.Parse(time.RFC3339, a)
	if err != nil {
		return false
	}
	tb, err := time.Parse(time.RFC3339, b)
	if err != nil {
		return false
	}
	return ta.Equal(tb)
}

func maintenanceWindowModelFromAPI(m map[string]any) maintenanceWindowModel {
	return maintenanceWindowModel{
		ID:             types.StringValue(strFromMap(m, "id")),
		Name:           types.StringValue(strFromMap(m, "name")),
		Reason:         types.StringValue(strFromMap(m, "reason")),
		TeamID:         nullableString(strFromMap(m, "team_id")),
		IntegrationIDs: stringSliceToList(stringSliceFromMap(m, "integration_ids")),
		StartsAt:       types.StringValue(strFromMap(m, "starts_at")),
		EndsAt:         types.StringValue(strFromMap(m, "ends_at")),
		CreatedAt:      types.StringValue(strFromMap(m, "created_at")),
		ProvisionedBy:  nullableString(strFromMap(m, "provisioned_by")),
		UpdatedAt:      types.StringValue(strFromMap(m, "updated_at")),
	}
}
