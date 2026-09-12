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

var _ datasource.DataSource = &EscalationChainsDataSource{}

type EscalationChainsDataSource struct{ client *client }

func NewEscalationChainsDataSource() datasource.DataSource { return &EscalationChainsDataSource{} }

type escalationChainsDataSourceModel struct {
	ID               types.String `tfsdk:"id"`
	EscalationChains types.List   `tfsdk:"escalation_chains"`
}

var escalationChainItemAttrTypes = map[string]attr.Type{
	"id":         types.StringType,
	"name":       types.StringType,
	"steps":      types.ListType{ElemType: types.ObjectType{AttrTypes: stepAttrTypes}},
	"created_at": types.StringType,
	"updated_at": types.StringType,
}

func (d *EscalationChainsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_escalation_chains"
}

func (d *EscalationChainsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Returns all escalation chains defined in nxs-anomaly.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true},
			"escalation_chains": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":   schema.StringAttribute{Computed: true},
						"name": schema.StringAttribute{Computed: true},
						"steps": schema.ListNestedAttribute{
							Computed: true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"id":               schema.StringAttribute{Computed: true},
									"kind":             schema.StringAttribute{Computed: true},
									"user_ids":         schema.ListAttribute{Computed: true, ElementType: types.StringType},
									"schedule_id":      schema.StringAttribute{Computed: true},
									"allow_uncovered":  schema.BoolAttribute{Computed: true},
									"team_id":          schema.StringAttribute{Computed: true},
									"fallback_to_all":  schema.BoolAttribute{Computed: true},
									"user_id":          schema.StringAttribute{Computed: true},
									"webhook_url":      schema.StringAttribute{Computed: true},
									"delay_minutes":    schema.Int64Attribute{Computed: true},
									"tracker_type":     schema.StringAttribute{Computed: true},
									"url":              schema.StringAttribute{Computed: true},
									"token":            schema.StringAttribute{Computed: true, Sensitive: true},
									"token_env":        schema.StringAttribute{Computed: true},
									"project":          schema.StringAttribute{Computed: true},
									"subject_template": schema.StringAttribute{Computed: true},
									"body_template":    schema.StringAttribute{Computed: true},
									"from_position":    schema.Int64Attribute{Computed: true},
									"max_repeat_count": schema.Int64Attribute{Computed: true},
									"cooldown_minutes": schema.Int64Attribute{Computed: true},
								},
							},
						},
						"created_at": schema.StringAttribute{Computed: true},
						"updated_at": schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *EscalationChainsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *EscalationChainsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	items, err := d.client.listAll(ctx, "/api/v1/escalation-chains")
	if err != nil {
		resp.Diagnostics.AddError("list escalation chains failed", err.Error())
		return
	}
	chainVals := make([]attr.Value, 0, len(items))
	for _, m := range items {
		obj, diags := escalationChainItemFromAPIVal(m)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		chainVals = append(chainVals, obj)
	}
	chainList, diags := types.ListValue(types.ObjectType{AttrTypes: escalationChainItemAttrTypes}, chainVals)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &escalationChainsDataSourceModel{
		ID:               types.StringValue("escalation_chains"),
		EscalationChains: chainList,
	})...)
}

func escalationChainItemFromAPIVal(m map[string]any) (types.Object, diag.Diagnostics) {
	var allDiags diag.Diagnostics
	chain := escalationChainModelFromAPI(m)
	obj, diags := types.ObjectValue(escalationChainItemAttrTypes, map[string]attr.Value{
		"id":         chain.ID,
		"name":       chain.Name,
		"steps":      chain.Steps,
		"created_at": chain.CreatedAt,
		"updated_at": chain.UpdatedAt,
	})
	allDiags.Append(diags...)
	return obj, allDiags
}
