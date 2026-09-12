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

var _ datasource.DataSource = &MaintenanceWindowDataSource{}
var _ datasource.DataSource = &MaintenanceWindowsDataSource{}

type MaintenanceWindowDataSource struct{ client *client }
type MaintenanceWindowsDataSource struct{ client *client }

func NewMaintenanceWindowDataSource() datasource.DataSource  { return &MaintenanceWindowDataSource{} }
func NewMaintenanceWindowsDataSource() datasource.DataSource { return &MaintenanceWindowsDataSource{} }

type maintenanceWindowDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Reason         types.String `tfsdk:"reason"`
	TeamID         types.String `tfsdk:"team_id"`
	IntegrationIDs types.List   `tfsdk:"integration_ids"`
	StartsAt       types.String `tfsdk:"starts_at"`
	EndsAt         types.String `tfsdk:"ends_at"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

type maintenanceWindowsDataSourceModel struct {
	ID                 types.String `tfsdk:"id"`
	MaintenanceWindows types.List   `tfsdk:"maintenance_windows"`
}

var maintenanceWindowItemAttrTypes = map[string]attr.Type{
	"id":              types.StringType,
	"name":            types.StringType,
	"reason":          types.StringType,
	"team_id":         types.StringType,
	"integration_ids": types.ListType{ElemType: types.StringType},
	"starts_at":       types.StringType,
	"ends_at":         types.StringType,
	"created_at":      types.StringType,
	"updated_at":      types.StringType,
}

func (d *MaintenanceWindowDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_maintenance_window"
}

func (d *MaintenanceWindowsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_maintenance_windows"
}

func (d *MaintenanceWindowDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Looks up a single nxs-anomaly maintenance window by ID or name.",
		Attributes: map[string]schema.Attribute{
			"id":              schema.StringAttribute{Optional: true, Computed: true, Description: "Object ID to look up. Supply either id or the name/username lookup attribute."},
			"name":            schema.StringAttribute{Optional: true, Computed: true},
			"reason":          schema.StringAttribute{Computed: true},
			"team_id":         schema.StringAttribute{Computed: true},
			"integration_ids": schema.ListAttribute{Computed: true, ElementType: types.StringType},
			"starts_at":       schema.StringAttribute{Computed: true},
			"ends_at":         schema.StringAttribute{Computed: true},
			"created_at":      schema.StringAttribute{Computed: true},
			"updated_at":      schema.StringAttribute{Computed: true},
		},
	}
}

func (d *MaintenanceWindowsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Returns all maintenance windows defined in nxs-anomaly.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Always set to \"maintenance_windows\".",
			},
			"maintenance_windows": schema.ListNestedAttribute{
				Computed:    true,
				Description: "List of all maintenance windows, past and future.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":              schema.StringAttribute{Computed: true},
						"name":            schema.StringAttribute{Computed: true},
						"reason":          schema.StringAttribute{Computed: true},
						"team_id":         schema.StringAttribute{Computed: true},
						"integration_ids": schema.ListAttribute{Computed: true, ElementType: types.StringType},
						"starts_at":       schema.StringAttribute{Computed: true},
						"ends_at":         schema.StringAttribute{Computed: true},
						"created_at":      schema.StringAttribute{Computed: true},
						"updated_at":      schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *MaintenanceWindowDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureDataSourceClient(req, resp)
}

func (d *MaintenanceWindowsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureDataSourceClient(req, resp)
}

func (d *MaintenanceWindowDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config maintenanceWindowDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var m map[string]any
	switch {
	case !config.ID.IsNull() && !config.ID.IsUnknown() && config.ID.ValueString() != "":
		result, err := d.client.get(ctx, "/api/v1/maintenance-windows/"+config.ID.ValueString())
		if err != nil {
			if isNotFound(err) {
				resp.Diagnostics.AddError("maintenance window not found",
					fmt.Sprintf("no maintenance window with id %q", config.ID.ValueString()))
				return
			}
			resp.Diagnostics.AddError("read maintenance window failed", err.Error())
			return
		}
		m = result
	case !config.Name.IsNull() && !config.Name.IsUnknown() && config.Name.ValueString() != "":
		items, err := d.client.listAll(ctx, "/api/v1/maintenance-windows")
		if err != nil {
			resp.Diagnostics.AddError("list maintenance windows failed", err.Error())
			return
		}
		m = lookupByName(items, config.Name.ValueString())
		if m == nil {
			resp.Diagnostics.AddError("maintenance window not found",
				fmt.Sprintf("no maintenance window with name %q", config.Name.ValueString()))
			return
		}
	default:
		resp.Diagnostics.AddError("missing lookup key", "provide either id or name")
		return
	}

	model := maintenanceWindowModelFromAPI(m)
	resp.Diagnostics.Append(resp.State.Set(ctx, &maintenanceWindowDataSourceModel{
		ID:             model.ID,
		Name:           model.Name,
		Reason:         model.Reason,
		TeamID:         model.TeamID,
		IntegrationIDs: model.IntegrationIDs,
		StartsAt:       model.StartsAt,
		EndsAt:         model.EndsAt,
		CreatedAt:      model.CreatedAt,
		UpdatedAt:      model.UpdatedAt,
	})...)
}

func (d *MaintenanceWindowsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	items, err := d.client.listAll(ctx, "/api/v1/maintenance-windows")
	if err != nil {
		resp.Diagnostics.AddError("list maintenance windows failed", err.Error())
		return
	}

	vals := make([]attr.Value, 0, len(items))
	for _, m := range items {
		obj, diags := maintenanceWindowItemFromAPIVal(m)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		vals = append(vals, obj)
	}

	list, diags := types.ListValue(types.ObjectType{AttrTypes: maintenanceWindowItemAttrTypes}, vals)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &maintenanceWindowsDataSourceModel{
		ID:                 types.StringValue("maintenance_windows"),
		MaintenanceWindows: list,
	})...)
}

func maintenanceWindowItemFromAPIVal(m map[string]any) (types.Object, diag.Diagnostics) {
	return types.ObjectValue(maintenanceWindowItemAttrTypes, map[string]attr.Value{
		"id":              types.StringValue(strFromMap(m, "id")),
		"name":            types.StringValue(strFromMap(m, "name")),
		"reason":          types.StringValue(strFromMap(m, "reason")),
		"team_id":         nullableString(strFromMap(m, "team_id")),
		"integration_ids": stringSliceToList(stringSliceFromMap(m, "integration_ids")),
		"starts_at":       types.StringValue(strFromMap(m, "starts_at")),
		"ends_at":         types.StringValue(strFromMap(m, "ends_at")),
		"created_at":      types.StringValue(strFromMap(m, "created_at")),
		"updated_at":      types.StringValue(strFromMap(m, "updated_at")),
	})
}
