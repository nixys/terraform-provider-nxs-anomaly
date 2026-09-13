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

var _ datasource.DataSource = &IntegrationsDataSource{}

type IntegrationsDataSource struct{ client *client }

func NewIntegrationsDataSource() datasource.DataSource { return &IntegrationsDataSource{} }

type integrationsDataSourceModel struct {
	ID           types.String `tfsdk:"id"`
	Type         types.String `tfsdk:"type"`
	Integrations types.List   `tfsdk:"integrations"`
}

var integrationItemAttrTypes = map[string]attr.Type{
	"id":                  types.StringType,
	"name":                types.StringType,
	"key":                 types.StringType,
	"type":                types.StringType,
	"source_type":         types.StringType,
	"group_by":            types.ListType{ElemType: types.StringType},
	"routes":              types.ListType{ElemType: types.ObjectType{AttrTypes: routeAttrTypes}},
	"notification_policy": types.ObjectType{AttrTypes: notificationPolicyAttrTypes},
	"templates":           types.MapType{ElemType: types.StringType},
	"team_id":             types.StringType,
	"kafka_topic":         types.StringType,
	"heartbeat":           types.ObjectType{AttrTypes: heartbeatAttrTypes},
	"legacy_pool":         types.ObjectType{AttrTypes: legacyPoolAttrTypes},
	"created_at":          types.StringType,
	"updated_at":          types.StringType,
}

func (d *IntegrationsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integrations"
}

func (d *IntegrationsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Returns integrations defined in nxs-anomaly, optionally filtered by type.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Set to the type filter value, or \"integrations\" when unfiltered.",
			},
			"type": schema.StringAttribute{
				Optional:    true,
				Description: "Filter by integration type (webhook, alertmanager, pagerduty, victorops, grafana-alerting).",
			},
			"integrations": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":          schema.StringAttribute{Computed: true},
						"name":        schema.StringAttribute{Computed: true},
						"key":         schema.StringAttribute{Computed: true},
						"type":        schema.StringAttribute{Computed: true},
						"source_type": schema.StringAttribute{Computed: true},
						"group_by":    schema.ListAttribute{Computed: true, ElementType: types.StringType},
						"routes": schema.ListNestedAttribute{
							Computed: true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"id":                  schema.StringAttribute{Computed: true},
									"name":                schema.StringAttribute{Computed: true},
									"match_type":          schema.StringAttribute{Computed: true},
									"is_default":          schema.BoolAttribute{Computed: true},
									"labels":              schema.MapAttribute{Computed: true, ElementType: types.StringType},
									"pattern":             schema.StringAttribute{Computed: true},
									"escalation_chain_id": schema.StringAttribute{Computed: true},
								},
							},
						},
						"notification_policy": notificationPolicyDataSourceSchema(),
						"templates":           schema.MapAttribute{Computed: true, ElementType: types.StringType},
						"team_id":             schema.StringAttribute{Computed: true},
						"kafka_topic":         schema.StringAttribute{Computed: true},
						"heartbeat":           heartbeatDataSourceSchema(),
						"legacy_pool":         legacyPoolDataSourceSchema(),
						"created_at":          schema.StringAttribute{Computed: true},
						"updated_at":          schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *IntegrationsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *IntegrationsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config integrationsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	items, err := d.client.listAll(ctx, "/api/v1/integrations")
	if err != nil {
		resp.Diagnostics.AddError("list integrations failed", err.Error())
		return
	}
	typeFilter := ""
	if !config.Type.IsNull() && !config.Type.IsUnknown() {
		typeFilter = config.Type.ValueString()
	}
	id := "integrations"
	if typeFilter != "" {
		id = typeFilter
	}
	intVals := make([]attr.Value, 0, len(items))
	for _, m := range items {
		if typeFilter != "" && strFromMap(m, "type") != typeFilter {
			continue
		}
		obj, diags := integrationItemFromAPIVal(m)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		intVals = append(intVals, obj)
	}
	intList, diags := types.ListValue(types.ObjectType{AttrTypes: integrationItemAttrTypes}, intVals)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &integrationsDataSourceModel{
		ID:           types.StringValue(id),
		Type:         config.Type,
		Integrations: intList,
	})...)
}

func integrationItemFromAPIVal(m map[string]any) (types.Object, diag.Diagnostics) {
	var allDiags diag.Diagnostics

	routeVals := make([]attr.Value, 0)
	for _, r := range mapSliceFromMap(m, "routes") {
		labelsRaw, _ := r["labels"].(map[string]any)
		labelsMap := make(map[string]attr.Value, len(labelsRaw))
		for k, v := range labelsRaw {
			labelsMap[k] = types.StringValue(fmt.Sprintf("%v", v))
		}
		labelsObj, diags := types.MapValue(types.StringType, labelsMap)
		allDiags.Append(diags...)
		if allDiags.HasError() {
			return types.ObjectUnknown(integrationItemAttrTypes), allDiags
		}

		rObj, diags := types.ObjectValue(routeAttrTypes, map[string]attr.Value{
			"id":                  types.StringValue(strFromMap(r, "id")),
			"name":                types.StringValue(strFromMap(r, "name")),
			"match_type":          types.StringValue(strDefault(strFromMap(r, "match_type"), "all")),
			"is_default":          types.BoolValue(boolFromMap(r, "is_default", false)),
			"labels":              labelsObj,
			"pattern":             types.StringValue(strFromMap(r, "pattern")),
			"escalation_chain_id": types.StringValue(strFromMap(r, "escalation_chain_id")),
		})
		allDiags.Append(diags...)
		if allDiags.HasError() {
			return types.ObjectUnknown(integrationItemAttrTypes), allDiags
		}
		routeVals = append(routeVals, rObj)
	}
	routeList, diags := types.ListValue(types.ObjectType{AttrTypes: routeAttrTypes}, routeVals)
	allDiags.Append(diags...)
	if allDiags.HasError() {
		return types.ObjectUnknown(integrationItemAttrTypes), allDiags
	}
	model := integrationModelFromAPI(m)

	obj, diags := types.ObjectValue(integrationItemAttrTypes, map[string]attr.Value{
		"id":                  types.StringValue(strFromMap(m, "id")),
		"name":                types.StringValue(strFromMap(m, "name")),
		"key":                 types.StringValue(strFromMap(m, "key")),
		"type":                types.StringValue(strFromMap(m, "type")),
		"source_type":         types.StringValue(strFromMap(m, "source_type")),
		"group_by":            stringSliceToList(stringSliceFromMap(m, "group_by")),
		"routes":              routeList,
		"notification_policy": model.NotificationPolicy,
		"templates":           model.Templates,
		"team_id":             model.TeamID,
		"kafka_topic":         model.KafkaTopic,
		"heartbeat":           model.Heartbeat,
		"legacy_pool":         model.LegacyPool,
		"created_at":          types.StringValue(strFromMap(m, "created_at")),
		"updated_at":          types.StringValue(strFromMap(m, "updated_at")),
	})
	allDiags.Append(diags...)
	return obj, allDiags
}
