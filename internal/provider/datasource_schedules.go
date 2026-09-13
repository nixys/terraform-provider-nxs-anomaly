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

var _ datasource.DataSource = &SchedulesDataSource{}

type SchedulesDataSource struct{ client *client }

func NewSchedulesDataSource() datasource.DataSource { return &SchedulesDataSource{} }

type schedulesDataSourceModel struct {
	ID        types.String `tfsdk:"id"`
	Schedules types.List   `tfsdk:"schedules"`
}

var scheduleItemAttrTypes = map[string]attr.Type{
	"id":                     types.StringType,
	"name":                   types.StringType,
	"timezone":               types.StringType,
	"team_id":                types.StringType,
	"enabled":                types.BoolType,
	"notify_on_shift_change": types.BoolType,
	"rotation":               types.ObjectType{AttrTypes: rotationAttrTypes},
	"shifts":                 types.ListType{ElemType: types.ObjectType{AttrTypes: shiftAttrTypes}},
	"created_at":             types.StringType,
	"updated_at":             types.StringType,
}

func (d *SchedulesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_schedules"
}

func (d *SchedulesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Returns all schedules defined in nxs-anomaly.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true},
			"schedules": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":                     schema.StringAttribute{Computed: true},
						"name":                   schema.StringAttribute{Computed: true},
						"timezone":               schema.StringAttribute{Computed: true},
						"team_id":                schema.StringAttribute{Computed: true},
						"enabled":                schema.BoolAttribute{Computed: true},
						"notify_on_shift_change": schema.BoolAttribute{Computed: true},
						"rotation":               rotationDataSourceSchema(),
						"shifts": schema.ListNestedAttribute{
							Computed: true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"id":         schema.StringAttribute{Computed: true},
									"user_id":    schema.StringAttribute{Computed: true},
									"start_at":   schema.StringAttribute{Computed: true},
									"end_at":     schema.StringAttribute{Computed: true},
									"recurrence": schema.StringAttribute{Computed: true},
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

func (d *SchedulesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *SchedulesDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	items, err := d.client.listAll(ctx, "/api/v1/schedules")
	if err != nil {
		resp.Diagnostics.AddError("list schedules failed", err.Error())
		return
	}
	schedVals := make([]attr.Value, 0, len(items))
	for _, m := range items {
		obj, diags := scheduleItemFromAPIVal(m)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		schedVals = append(schedVals, obj)
	}
	schedList, diags := types.ListValue(types.ObjectType{AttrTypes: scheduleItemAttrTypes}, schedVals)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &schedulesDataSourceModel{
		ID:        types.StringValue("schedules"),
		Schedules: schedList,
	})...)
}

func scheduleItemFromAPIVal(m map[string]any) (types.Object, diag.Diagnostics) {
	var allDiags diag.Diagnostics

	shifts := mapSliceFromMap(m, "shifts")
	shiftVals := make([]attr.Value, 0, len(shifts))
	for _, s := range shifts {
		sObj, diags := types.ObjectValue(shiftAttrTypes, map[string]attr.Value{
			"id":         types.StringValue(strFromMap(s, "id")),
			"user_id":    types.StringValue(strFromMap(s, "user_id")),
			"start_at":   types.StringValue(strFromMap(s, "start_at")),
			"end_at":     types.StringValue(strFromMap(s, "end_at")),
			"recurrence": types.StringValue(strFromMap(s, "recurrence")),
		})
		allDiags.Append(diags...)
		if allDiags.HasError() {
			return types.ObjectUnknown(scheduleItemAttrTypes), allDiags
		}
		shiftVals = append(shiftVals, sObj)
	}
	shiftList, diags := types.ListValue(types.ObjectType{AttrTypes: shiftAttrTypes}, shiftVals)
	allDiags.Append(diags...)
	if allDiags.HasError() {
		return types.ObjectUnknown(scheduleItemAttrTypes), allDiags
	}

	teamID := types.StringValue(strFromMap(m, "team_id"))
	if strFromMap(m, "team_id") == "" {
		teamID = types.StringNull()
	}
	rotation := rotationModelFromAPI(m["rotation"], strDefault(strFromMap(m, "timezone"), "UTC"))

	obj, diags := types.ObjectValue(scheduleItemAttrTypes, map[string]attr.Value{
		"id":                     types.StringValue(strFromMap(m, "id")),
		"name":                   types.StringValue(strFromMap(m, "name")),
		"timezone":               types.StringValue(strFromMap(m, "timezone")),
		"team_id":                teamID,
		"enabled":                types.BoolValue(boolFromMap(m, "enabled", true)),
		"notify_on_shift_change": types.BoolValue(boolFromMap(m, "notify_on_shift_change", false)),
		"rotation":               rotation,
		"shifts":                 shiftList,
		"created_at":             types.StringValue(strFromMap(m, "created_at")),
		"updated_at":             types.StringValue(strFromMap(m, "updated_at")),
	})
	allDiags.Append(diags...)
	return obj, allDiags
}
