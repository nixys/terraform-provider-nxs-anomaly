package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &TeamResource{}
var _ resource.ResourceWithImportState = &TeamResource{}

type TeamResource struct{ client *client }

func NewTeamResource() resource.Resource { return &TeamResource{} }

func (r *TeamResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team"
}

type teamModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	MemberIDs     types.List   `tfsdk:"member_ids"`
	CreatedAt     types.String `tfsdk:"created_at"`
	ProvisionedBy types.String `tfsdk:"provisioned_by"`
	UpdatedAt     types.String `tfsdk:"updated_at"`
}

func (r *TeamResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a nxs-anomaly team.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Team name.",
			},
			"member_ids": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "List of user IDs that are members of this team.",
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

func (r *TeamResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *TeamResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan teamModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"name":       plan.Name.ValueString(),
		"member_ids": stringListToAny(ctx, plan.MemberIDs),
	}

	result, err := r.client.post(ctx, "/api/v1/teams", body)
	if err != nil {
		resp.Diagnostics.AddError("create team failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, teamModelFromAPI(result))...)
}

func (r *TeamResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state teamModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.get(ctx, "/api/v1/teams/"+state.ID.ValueString())
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read team failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, teamModelFromAPI(result))...)
}

func (r *TeamResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan teamModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"name":       plan.Name.ValueString(),
		"member_ids": stringListToAny(ctx, plan.MemberIDs),
	}

	result, err := r.client.put(ctx, "/api/v1/teams/"+plan.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("update team failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, teamModelFromAPI(result))...)
}

func (r *TeamResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state teamModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.delete(ctx, "/api/v1/teams/"+state.ID.ValueString()); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete team failed", err.Error())
	}
}

func (r *TeamResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func teamModelFromAPI(m map[string]any) teamModel {
	memberIDs := stringSliceFromMap(m, "member_ids")
	memberList := stringSliceToList(memberIDs)
	return teamModel{
		ID:            types.StringValue(strFromMap(m, "id")),
		Name:          types.StringValue(strFromMap(m, "name")),
		MemberIDs:     memberList,
		CreatedAt:     types.StringValue(strFromMap(m, "created_at")),
		ProvisionedBy: nullableString(strFromMap(m, "provisioned_by")),
		UpdatedAt:     types.StringValue(strFromMap(m, "updated_at")),
	}
}

func stringListToAny(ctx context.Context, list types.List) []any {
	if list.IsNull() || list.IsUnknown() {
		return nil
	}
	var strs []types.String
	list.ElementsAs(ctx, &strs, false)
	out := make([]any, len(strs))
	for i, s := range strs {
		out[i] = s.ValueString()
	}
	return out
}

func stringSliceToList(ss []string) types.List {
	if ss == nil {
		list, _ := types.ListValueFrom(context.Background(), types.StringType, []string{})
		return list
	}
	list, _ := types.ListValueFrom(context.Background(), types.StringType, ss)
	return list
}
