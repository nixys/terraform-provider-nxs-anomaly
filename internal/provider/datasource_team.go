package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &TeamDataSource{}

type TeamDataSource struct{ client *client }

func NewTeamDataSource() datasource.DataSource { return &TeamDataSource{} }

type teamDataSourceModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	MemberIDs types.List   `tfsdk:"member_ids"`
	CreatedAt types.String `tfsdk:"created_at"`
	UpdatedAt types.String `tfsdk:"updated_at"`
}

func (d *TeamDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team"
}

func (d *TeamDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Looks up a single nxs-anomaly team by ID or name.",
		Attributes: map[string]schema.Attribute{
			"id":         schema.StringAttribute{Optional: true, Computed: true, Description: "Object ID to look up. Supply either id or the name/username lookup attribute."},
			"name":       schema.StringAttribute{Optional: true, Computed: true},
			"member_ids": schema.ListAttribute{Computed: true, ElementType: types.StringType},
			"created_at": schema.StringAttribute{Computed: true},
			"updated_at": schema.StringAttribute{Computed: true},
		},
	}
}

func (d *TeamDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *TeamDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config teamDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var m map[string]any
	if !config.ID.IsNull() && !config.ID.IsUnknown() && config.ID.ValueString() != "" {
		result, err := d.client.get(ctx, "/api/v1/teams/"+config.ID.ValueString())
		if err != nil {
			if isNotFound(err) {
				resp.Diagnostics.AddError("team not found", fmt.Sprintf("no team with id %q", config.ID.ValueString()))
				return
			}
			resp.Diagnostics.AddError("read team failed", err.Error())
			return
		}
		m = result
	} else if !config.Name.IsNull() && !config.Name.IsUnknown() && config.Name.ValueString() != "" {
		items, err := d.client.listAll(ctx, "/api/v1/teams")
		if err != nil {
			resp.Diagnostics.AddError("list teams failed", err.Error())
			return
		}
		matched, matches := lookupByName(items, config.Name.ValueString())
		if matches > 1 {
			resp.Diagnostics.AddError(ambiguousNameError("team", config.Name.ValueString(), matches))
			return
		}
		m = matched
		if m == nil {
			resp.Diagnostics.AddError("team not found", fmt.Sprintf("no team with name %q", config.Name.ValueString()))
			return
		}
	} else {
		resp.Diagnostics.AddError("missing lookup key", "provide either id or name")
		return
	}

	model := teamModelFromAPI(m)
	resp.Diagnostics.Append(resp.State.Set(ctx, &teamDataSourceModel{
		ID:        model.ID,
		Name:      model.Name,
		MemberIDs: model.MemberIDs,
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
	})...)
}
