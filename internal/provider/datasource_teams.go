package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &TeamsDataSource{}

type TeamsDataSource struct{ client *client }

func NewTeamsDataSource() datasource.DataSource { return &TeamsDataSource{} }

type teamsDataSourceModel struct {
	ID    types.String `tfsdk:"id"`
	Teams types.List   `tfsdk:"teams"`
}

var teamItemAttrTypes = map[string]attr.Type{
	"id":         types.StringType,
	"name":       types.StringType,
	"member_ids": types.ListType{ElemType: types.StringType},
	"created_at": types.StringType,
	"updated_at": types.StringType,
}

func (d *TeamsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_teams"
}

func (d *TeamsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Returns all teams defined in nxs-anomaly.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Always set to \"teams\".",
			},
			"teams": schema.ListNestedAttribute{
				Computed:    true,
				Description: "List of all teams.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":   schema.StringAttribute{Computed: true},
						"name": schema.StringAttribute{Computed: true},
						"member_ids": schema.ListAttribute{
							Computed:    true,
							ElementType: types.StringType,
						},
						"created_at": schema.StringAttribute{Computed: true},
						"updated_at": schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *TeamsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client)
	if !ok {
		resp.Diagnostics.AddError("unexpected provider data type", fmt.Sprintf("got %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *TeamsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	items, err := d.client.listAll(ctx, "/api/v1/teams")
	if err != nil {
		resp.Diagnostics.AddError("list teams failed", err.Error())
		return
	}

	teamVals := make([]attr.Value, 0, len(items))
	for _, m := range items {
		obj, diags := teamItemFromAPIVal(m)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		teamVals = append(teamVals, obj)
	}

	teamList, diags := types.ListValue(types.ObjectType{AttrTypes: teamItemAttrTypes}, teamVals)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &teamsDataSourceModel{
		ID:    types.StringValue("teams"),
		Teams: teamList,
	})...)
}

func teamItemFromAPIVal(m map[string]any) (types.Object, diag.Diagnostics) {
	memberIDs := stringSliceFromMap(m, "member_ids")
	memberList := stringSliceToList(memberIDs)

	return types.ObjectValue(teamItemAttrTypes, map[string]attr.Value{
		"id":         types.StringValue(strFromMap(m, "id")),
		"name":       types.StringValue(strFromMap(m, "name")),
		"member_ids": memberList,
		"created_at": types.StringValue(strFromMap(m, "created_at")),
		"updated_at": types.StringValue(strFromMap(m, "updated_at")),
	})
}
