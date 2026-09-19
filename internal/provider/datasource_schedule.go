package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &ScheduleDataSource{}

type ScheduleDataSource struct{ client *client }

func NewScheduleDataSource() datasource.DataSource { return &ScheduleDataSource{} }

type scheduleDataSourceModel struct {
	ID                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	Timezone            types.String `tfsdk:"timezone"`
	TeamID              types.String `tfsdk:"team_id"`
	Enabled             types.Bool   `tfsdk:"enabled"`
	NotifyOnShiftChange types.Bool   `tfsdk:"notify_on_shift_change"`
	Rotation            types.Object `tfsdk:"rotation"`
	Shifts              types.List   `tfsdk:"shifts"`
	CreatedAt           types.String `tfsdk:"created_at"`
	UpdatedAt           types.String `tfsdk:"updated_at"`
}

func (d *ScheduleDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_schedule"
}

func (d *ScheduleDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Looks up a single nxs-anomaly schedule by ID or name.",
		Attributes: map[string]schema.Attribute{
			"id":                     schema.StringAttribute{Optional: true, Computed: true, Description: "Object ID to look up. Supply either id or the name/username lookup attribute."},
			"name":                   schema.StringAttribute{Optional: true, Computed: true},
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
	}
}

func (d *ScheduleDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ScheduleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config scheduleDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var m map[string]any
	if !config.ID.IsNull() && !config.ID.IsUnknown() && config.ID.ValueString() != "" {
		result, err := d.client.get(ctx, "/api/v1/schedules/"+config.ID.ValueString())
		if err != nil {
			if isNotFound(err) {
				resp.Diagnostics.AddError("schedule not found", fmt.Sprintf("no schedule with id %q", config.ID.ValueString()))
				return
			}
			resp.Diagnostics.AddError("read schedule failed", err.Error())
			return
		}
		m = result
	} else if !config.Name.IsNull() && !config.Name.IsUnknown() && config.Name.ValueString() != "" {
		items, err := d.client.listAll(ctx, "/api/v1/schedules")
		if err != nil {
			resp.Diagnostics.AddError("list schedules failed", err.Error())
			return
		}
		matched, matches := lookupByName(items, config.Name.ValueString())
		if matches > 1 {
			resp.Diagnostics.AddError(ambiguousNameError("schedule", config.Name.ValueString(), matches))
			return
		}
		m = matched
		if m == nil {
			resp.Diagnostics.AddError("schedule not found", fmt.Sprintf("no schedule with name %q", config.Name.ValueString()))
			return
		}
	} else {
		resp.Diagnostics.AddError("missing lookup key", "provide either id or name")
		return
	}

	model := scheduleModelFromAPI(m)
	resp.Diagnostics.Append(resp.State.Set(ctx, &scheduleDataSourceModel{
		ID:                  model.ID,
		Name:                model.Name,
		Timezone:            model.Timezone,
		TeamID:              model.TeamID,
		Enabled:             model.Enabled,
		NotifyOnShiftChange: model.NotifyOnShiftChange,
		Rotation:            model.Rotation,
		Shifts:              model.Shifts,
		CreatedAt:           model.CreatedAt,
		UpdatedAt:           model.UpdatedAt,
	})...)
}

func rotationDataSourceSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{Computed: true, Attributes: map[string]schema.Attribute{
		"enabled":          schema.BoolAttribute{Computed: true},
		"start_at":         schema.StringAttribute{Computed: true},
		"handoff_interval": schema.Int64Attribute{Computed: true},
		"handoff_unit":     schema.StringAttribute{Computed: true},
		"participant_ids":  schema.ListAttribute{Computed: true, ElementType: types.StringType},
		"restriction": schema.SingleNestedAttribute{Computed: true, Attributes: map[string]schema.Attribute{
			"start": schema.StringAttribute{Computed: true},
			"end":   schema.StringAttribute{Computed: true},
			"days":  schema.ListAttribute{Computed: true, ElementType: types.StringType},
		}},
	}}
}
