package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &OnCallDataSource{}

type OnCallDataSource struct{ client *client }

func NewOnCallDataSource() datasource.DataSource { return &OnCallDataSource{} }

type onCallDataSourceModel struct {
	ID    types.String `tfsdk:"id"`
	At    types.String `tfsdk:"at"`
	Items types.List   `tfsdk:"items"`
}

var onCallEntryAttrTypes = map[string]attr.Type{
	"user_id":       types.StringType,
	"name":          types.StringType,
	"username":      types.StringType,
	"exists":        types.BoolType,
	"schedule_id":   types.StringType,
	"schedule_name": types.StringType,
	"source":        types.StringType,
}

func (d *OnCallDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_on_call"
}

func (d *OnCallDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Who is on call right now across every enabled schedule. This is a live " +
			"reading, not configuration: it changes without any Terraform run.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Always set to \"on_call\".",
			},
			"at": schema.StringAttribute{
				Computed:    true,
				Description: "Instant the roster was resolved at.",
			},
			"items": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"user_id":  schema.StringAttribute{Computed: true},
						"name":     schema.StringAttribute{Computed: true},
						"username": schema.StringAttribute{Computed: true},
						"exists": schema.BoolAttribute{
							Computed: true,
							Description: "False when the schedule names a user that no longer " +
								"exists — a coverage hole, reported rather than hidden.",
						},
						"schedule_id":   schema.StringAttribute{Computed: true},
						"schedule_name": schema.StringAttribute{Computed: true},
						"source": schema.StringAttribute{
							Computed:    true,
							Description: "Which layer put them on call: override, rotation or shift.",
						},
					},
				},
			},
		},
	}
}

func (d *OnCallDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureDataSourceClient(req, resp)
}

func (d *OnCallDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	result, err := d.client.get(ctx, "/api/v1/on-call")
	if err != nil {
		resp.Diagnostics.AddError("read on-call roster failed", err.Error())
		return
	}

	entries := mapSliceFromMap(result, "items")
	vals := make([]attr.Value, 0, len(entries))
	for _, e := range entries {
		obj, diags := types.ObjectValue(onCallEntryAttrTypes, map[string]attr.Value{
			"user_id":       types.StringValue(strFromMap(e, "user_id")),
			"name":          types.StringValue(strFromMap(e, "name")),
			"username":      types.StringValue(strFromMap(e, "username")),
			"exists":        types.BoolValue(boolFromMap(e, "exists", false)),
			"schedule_id":   types.StringValue(strFromMap(e, "schedule_id")),
			"schedule_name": types.StringValue(strFromMap(e, "schedule_name")),
			"source":        types.StringValue(strFromMap(e, "source")),
		})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		vals = append(vals, obj)
	}

	list, diags := types.ListValue(types.ObjectType{AttrTypes: onCallEntryAttrTypes}, vals)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &onCallDataSourceModel{
		ID:    types.StringValue("on_call"),
		At:    types.StringValue(strFromMap(result, "at")),
		Items: list,
	})...)
}
