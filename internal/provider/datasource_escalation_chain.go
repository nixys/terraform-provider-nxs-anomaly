package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &EscalationChainDataSource{}

type EscalationChainDataSource struct{ client *client }

func NewEscalationChainDataSource() datasource.DataSource { return &EscalationChainDataSource{} }

type escalationChainDataSourceModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Steps     types.List   `tfsdk:"steps"`
	CreatedAt types.String `tfsdk:"created_at"`
	UpdatedAt types.String `tfsdk:"updated_at"`
}

func (d *EscalationChainDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_escalation_chain"
}

func (d *EscalationChainDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Looks up a single nxs-anomaly escalation chain by ID or name.",
		Attributes: map[string]schema.Attribute{
			"id":   schema.StringAttribute{Optional: true, Computed: true, Description: "Object ID to look up. Supply either id or the name/username lookup attribute."},
			"name": schema.StringAttribute{Optional: true, Computed: true},
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
						"headers":          schema.MapAttribute{Computed: true, Sensitive: true, ElementType: types.StringType},
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
	}
}

func (d *EscalationChainDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *EscalationChainDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config escalationChainDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var m map[string]any
	if !config.ID.IsNull() && !config.ID.IsUnknown() && config.ID.ValueString() != "" {
		result, err := d.client.get(ctx, "/api/v1/escalation-chains/"+config.ID.ValueString())
		if err != nil {
			if isNotFound(err) {
				resp.Diagnostics.AddError("escalation chain not found", fmt.Sprintf("no chain with id %q", config.ID.ValueString()))
				return
			}
			resp.Diagnostics.AddError("read escalation chain failed", err.Error())
			return
		}
		m = result
	} else if !config.Name.IsNull() && !config.Name.IsUnknown() && config.Name.ValueString() != "" {
		items, err := d.client.listAll(ctx, "/api/v1/escalation-chains")
		if err != nil {
			resp.Diagnostics.AddError("list escalation chains failed", err.Error())
			return
		}
		matched, matches := lookupByName(items, config.Name.ValueString())
		if matches > 1 {
			resp.Diagnostics.AddError(ambiguousNameError("escalation chain", config.Name.ValueString(), matches))
			return
		}
		m = matched
		if m == nil {
			resp.Diagnostics.AddError("escalation chain not found", fmt.Sprintf("no chain with name %q", config.Name.ValueString()))
			return
		}
	} else {
		resp.Diagnostics.AddError("missing lookup key", "provide either id or name")
		return
	}

	model := escalationChainModelFromAPI(m)
	resp.Diagnostics.Append(resp.State.Set(ctx, &escalationChainDataSourceModel{
		ID:        model.ID,
		Name:      model.Name,
		Steps:     model.Steps,
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
	})...)
}
