package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &ScheduleCoverageDataSource{}

type ScheduleCoverageDataSource struct{ client *client }

func NewScheduleCoverageDataSource() datasource.DataSource { return &ScheduleCoverageDataSource{} }

type scheduleCoverageDataSourceModel struct {
	ID                types.String `tfsdk:"id"`
	CheckedAt         types.String `tfsdk:"checked_at"`
	WindowDays        types.Int64  `tfsdk:"window_days"`
	SchedulesTotal    types.Int64  `tfsdk:"schedules_total"`
	SchedulesDegraded types.Int64  `tfsdk:"schedules_degraded"`
	DegradedAttached  types.Int64  `tfsdk:"degraded_attached"`
	Items             types.List   `tfsdk:"items"`
}

// segmentAttrTypes mirrors the engine's segmentJSON, shared by the coverage and
// preview reports.
var segmentAttrTypes = map[string]attr.Type{
	"start":            types.StringType,
	"end":              types.StringType,
	"user_ids":         types.ListType{ElemType: types.StringType},
	"source":           types.StringType,
	"duration_seconds": types.Int64Type,
}

var entityRefAttrTypes = map[string]attr.Type{
	"id":   types.StringType,
	"name": types.StringType,
}

var coverageItemAttrTypes = map[string]attr.Type{
	"schedule_id":   types.StringType,
	"name":          types.StringType,
	"disabled":      types.BoolType,
	"gap_count":     types.Int64Type,
	"unknown_users": types.ListType{ElemType: types.StringType},
	"attached_to":   types.ListType{ElemType: types.ObjectType{AttrTypes: entityRefAttrTypes}},
	"acknowledged":  types.BoolType,
	"first_gap":     types.ObjectType{AttrTypes: segmentAttrTypes},
}

func (d *ScheduleCoverageDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_schedule_coverage"
}

func (d *ScheduleCoverageDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Standing coverage report across all schedules. Only schedules with a gap, " +
			"a participant that no longer exists, or that are disabled are listed at all.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Always set to \"schedule_coverage\".",
			},
			"checked_at":  schema.StringAttribute{Computed: true},
			"window_days": schema.Int64Attribute{Computed: true},
			"schedules_total": schema.Int64Attribute{
				Computed:    true,
				Description: "Counted after team scoping, like items.",
			},
			"schedules_degraded": schema.Int64Attribute{Computed: true},
			"degraded_attached": schema.Int64Attribute{
				Computed:    true,
				Description: "Degraded schedules an escalation chain actually pages through.",
			},
			"items": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"schedule_id":   schema.StringAttribute{Computed: true},
						"name":          schema.StringAttribute{Computed: true},
						"disabled":      schema.BoolAttribute{Computed: true},
						"gap_count":     schema.Int64Attribute{Computed: true},
						"unknown_users": schema.ListAttribute{Computed: true, ElementType: types.StringType},
						"attached_to": schema.ListNestedAttribute{
							Computed:    true,
							Description: "Escalation chains that page through this schedule.",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"id":   schema.StringAttribute{Computed: true},
									"name": schema.StringAttribute{Computed: true},
								},
							},
						},
						"acknowledged": schema.BoolAttribute{
							Computed: true,
							Description: "Every referencing step set allow_uncovered, i.e. the gap " +
								"is an accepted decision rather than an oversight.",
						},
						"first_gap": segmentDataSourceSchema("Earliest gap in the window; null when there is none."),
					},
				},
			},
		},
	}
}

func (d *ScheduleCoverageDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureDataSourceClient(req, resp)
}

func (d *ScheduleCoverageDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	result, err := d.client.get(ctx, "/api/v1/schedules/coverage")
	if err != nil {
		resp.Diagnostics.AddError("read schedule coverage failed", err.Error())
		return
	}

	items := mapSliceFromMap(result, "items")
	vals := make([]attr.Value, 0, len(items))
	for _, item := range items {
		refs := mapSliceFromMap(item, "attached_to")
		refVals := make([]attr.Value, 0, len(refs))
		for _, r := range refs {
			obj, diags := types.ObjectValue(entityRefAttrTypes, map[string]attr.Value{
				"id":   types.StringValue(strFromMap(r, "id")),
				"name": types.StringValue(strFromMap(r, "name")),
			})
			resp.Diagnostics.Append(diags...)
			refVals = append(refVals, obj)
		}
		refList, diags := types.ListValue(types.ObjectType{AttrTypes: entityRefAttrTypes}, refVals)
		resp.Diagnostics.Append(diags...)

		firstGap := types.ObjectNull(segmentAttrTypes)
		if gap, ok := item["first_gap"].(map[string]any); ok && gap != nil {
			firstGap, diags = segmentObject(gap)
			resp.Diagnostics.Append(diags...)
		}

		obj, diags := types.ObjectValue(coverageItemAttrTypes, map[string]attr.Value{
			"schedule_id":   types.StringValue(strFromMap(item, "schedule_id")),
			"name":          types.StringValue(strFromMap(item, "name")),
			"disabled":      types.BoolValue(boolFromMap(item, "disabled", false)),
			"gap_count":     types.Int64Value(int64FromMap(item, "gap_count", 0)),
			"unknown_users": stringSliceToList(stringSliceFromMap(item, "unknown_users")),
			"attached_to":   refList,
			"acknowledged":  types.BoolValue(boolFromMap(item, "acknowledged", false)),
			"first_gap":     firstGap,
		})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		vals = append(vals, obj)
	}

	list, diags := types.ListValue(types.ObjectType{AttrTypes: coverageItemAttrTypes}, vals)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &scheduleCoverageDataSourceModel{
		ID:                types.StringValue("schedule_coverage"),
		CheckedAt:         types.StringValue(strFromMap(result, "checked_at")),
		WindowDays:        types.Int64Value(int64FromMap(result, "window_days", 0)),
		SchedulesTotal:    types.Int64Value(int64FromMap(result, "schedules_total", 0)),
		SchedulesDegraded: types.Int64Value(int64FromMap(result, "schedules_degraded", 0)),
		DegradedAttached:  types.Int64Value(int64FromMap(result, "degraded_attached", 0)),
		Items:             list,
	})...)
}

func segmentDataSourceSchema(description string) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Computed:    true,
		Description: description,
		Attributes: map[string]schema.Attribute{
			"start":            schema.StringAttribute{Computed: true},
			"end":              schema.StringAttribute{Computed: true},
			"user_ids":         schema.ListAttribute{Computed: true, ElementType: types.StringType},
			"source":           schema.StringAttribute{Computed: true},
			"duration_seconds": schema.Int64Attribute{Computed: true},
		},
	}
}

func segmentListDataSourceSchema(description string) schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		Computed:    true,
		Description: description,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"start":            schema.StringAttribute{Computed: true},
				"end":              schema.StringAttribute{Computed: true},
				"user_ids":         schema.ListAttribute{Computed: true, ElementType: types.StringType},
				"source":           schema.StringAttribute{Computed: true},
				"duration_seconds": schema.Int64Attribute{Computed: true},
			},
		},
	}
}

func segmentObject(m map[string]any) (types.Object, diag.Diagnostics) {
	return types.ObjectValue(segmentAttrTypes, map[string]attr.Value{
		"start":            types.StringValue(strFromMap(m, "start")),
		"end":              types.StringValue(strFromMap(m, "end")),
		"user_ids":         stringSliceToList(stringSliceFromMap(m, "user_ids")),
		"source":           types.StringValue(strFromMap(m, "source")),
		"duration_seconds": types.Int64Value(int64FromMap(m, "duration_seconds", 0)),
	})
}
