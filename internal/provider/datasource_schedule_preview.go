package provider

import (
	"context"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &SchedulePreviewDataSource{}

type SchedulePreviewDataSource struct{ client *client }

func NewSchedulePreviewDataSource() datasource.DataSource { return &SchedulePreviewDataSource{} }

type schedulePreviewDataSourceModel struct {
	ID            types.String  `tfsdk:"id"`
	ScheduleID    types.String  `tfsdk:"schedule_id"`
	From          types.String  `tfsdk:"from"`
	To            types.String  `tfsdk:"to"`
	Timezone      types.String  `tfsdk:"timezone"`
	Segments      types.List    `tfsdk:"segments"`
	Gaps          types.List    `tfsdk:"gaps"`
	Overlaps      types.List    `tfsdk:"overlaps"`
	Warnings      types.List    `tfsdk:"warnings"`
	CoverageRatio types.Float64 `tfsdk:"coverage_ratio"`
	Participants  types.List    `tfsdk:"participants"`
	UnknownUsers  types.List    `tfsdk:"unknown_users"`
}

func (d *SchedulePreviewDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_schedule_preview"
}

func (d *SchedulePreviewDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Resolves a schedule into a concrete timeline over a window: who is on call " +
			"when, and where the holes are. Useful as a plan-time check that a schedule about to " +
			"be attached to an escalation chain actually covers anybody.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Set to the schedule ID.",
			},
			"schedule_id": schema.StringAttribute{Required: true},
			"from": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Window start (RFC3339). Defaults to the API's own window.",
			},
			"to": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Window end (RFC3339). Defaults to the API's own window.",
			},
			"timezone": schema.StringAttribute{Computed: true},
			"segments": segmentListDataSourceSchema("Intervals with somebody on call."),
			"gaps":     segmentListDataSourceSchema("Intervals nobody is on call for."),
			"overlaps": segmentListDataSourceSchema("Intervals with more than one layer active."),
			"warnings": schema.ListAttribute{Computed: true, ElementType: types.StringType},
			"coverage_ratio": schema.Float64Attribute{
				Computed:    true,
				Description: "Covered fraction of the window, 0..1.",
			},
			"participants": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":   schema.StringAttribute{Computed: true},
						"name": schema.StringAttribute{Computed: true},
					},
				},
			},
			"unknown_users": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Users the schedule names that no longer exist; their slots page nobody.",
			},
		},
	}
}

func (d *SchedulePreviewDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureDataSourceClient(req, resp)
}

func (d *SchedulePreviewDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config schedulePreviewDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	query := url.Values{}
	if !config.From.IsNull() && !config.From.IsUnknown() && config.From.ValueString() != "" {
		query.Set("from", config.From.ValueString())
	}
	if !config.To.IsNull() && !config.To.IsUnknown() && config.To.ValueString() != "" {
		query.Set("to", config.To.ValueString())
	}
	path := "/api/v1/schedules/" + config.ScheduleID.ValueString() + "/preview"
	if len(query) > 0 {
		path += "?" + query.Encode()
	}

	result, err := d.client.get(ctx, path)
	if err != nil {
		resp.Diagnostics.AddError("read schedule preview failed", err.Error())
		return
	}

	segments := d.segmentList(result, "segments", resp)
	gaps := d.segmentList(result, "gaps", resp)
	overlaps := d.segmentList(result, "overlaps", resp)
	if resp.Diagnostics.HasError() {
		return
	}

	rawParticipants := mapSliceFromMap(result, "participants")
	participantVals := make([]attr.Value, 0, len(rawParticipants))
	for _, p := range rawParticipants {
		obj, diags := types.ObjectValue(entityRefAttrTypes, map[string]attr.Value{
			"id":   types.StringValue(strFromMap(p, "id")),
			"name": types.StringValue(strFromMap(p, "name")),
		})
		resp.Diagnostics.Append(diags...)
		participantVals = append(participantVals, obj)
	}
	participants, diags := types.ListValue(types.ObjectType{AttrTypes: entityRefAttrTypes}, participantVals)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &schedulePreviewDataSourceModel{
		ID:         types.StringValue(strFromMap(result, "schedule_id")),
		ScheduleID: config.ScheduleID,
		// The window is echoed back as the practitioner wrote it: the engine
		// normalises to UTC, and a configured attribute that comes back in a
		// different notation reads as the provider answering a question it was
		// not asked.
		From:          configuredOr(config.From, strFromMap(result, "from")),
		To:            configuredOr(config.To, strFromMap(result, "to")),
		Timezone:      types.StringValue(strFromMap(result, "timezone")),
		Segments:      segments,
		Gaps:          gaps,
		Overlaps:      overlaps,
		Warnings:      stringSliceToList(stringSliceFromMap(result, "warnings")),
		CoverageRatio: types.Float64Value(float64FromMap(result, "coverage_ratio", 0)),
		Participants:  participants,
		UnknownUsers:  stringSliceToList(stringSliceFromMap(result, "unknown_users")),
	})...)
}

func configuredOr(configured types.String, apiValue string) types.String {
	if !configured.IsNull() && !configured.IsUnknown() && configured.ValueString() != "" {
		return configured
	}
	return types.StringValue(apiValue)
}

func (d *SchedulePreviewDataSource) segmentList(result map[string]any, key string, resp *datasource.ReadResponse) types.List {
	raw := mapSliceFromMap(result, key)
	vals := make([]attr.Value, 0, len(raw))
	for _, s := range raw {
		obj, diags := segmentObject(s)
		resp.Diagnostics.Append(diags...)
		vals = append(vals, obj)
	}
	list, diags := types.ListValue(types.ObjectType{AttrTypes: segmentAttrTypes}, vals)
	resp.Diagnostics.Append(diags...)
	return list
}
