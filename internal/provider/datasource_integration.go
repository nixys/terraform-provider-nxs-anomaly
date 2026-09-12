package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &IntegrationDataSource{}

type IntegrationDataSource struct{ client *client }

func NewIntegrationDataSource() datasource.DataSource { return &IntegrationDataSource{} }

type integrationDataSourceModel struct {
	ID                 types.String `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	Key                types.String `tfsdk:"key"`
	Type               types.String `tfsdk:"type"`
	SourceType         types.String `tfsdk:"source_type"`
	GroupBy            types.List   `tfsdk:"group_by"`
	Routes             types.List   `tfsdk:"routes"`
	NotificationPolicy types.Object `tfsdk:"notification_policy"`
	Templates          types.Map    `tfsdk:"templates"`
	Pipeline           types.String `tfsdk:"pipeline"`
	ProvisionedBy      types.String `tfsdk:"provisioned_by"`
	TeamID             types.String `tfsdk:"team_id"`
	KafkaTopic         types.String `tfsdk:"kafka_topic"`
	Heartbeat          types.Object `tfsdk:"heartbeat"`
	LegacyPool         types.Object `tfsdk:"legacy_pool"`
	CreatedAt          types.String `tfsdk:"created_at"`
	UpdatedAt          types.String `tfsdk:"updated_at"`
}

func (d *IntegrationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration"
}

func (d *IntegrationDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Looks up a single nxs-anomaly integration by ID or name.",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Optional: true, Computed: true, Description: "Object ID to look up. Supply either id or the name/username lookup attribute."},
			"name":        schema.StringAttribute{Optional: true, Computed: true},
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
			"pipeline": schema.StringAttribute{
				Computed: true,
				Description: "Ingest pipeline as a JSON array of stages. Runs after route selection, " +
					"so it shapes deduplication, the stored alert and the notification text but never " +
					"moves which escalation chain pages.",
			},
			"provisioned_by": schema.StringAttribute{
				Computed:    true,
				Description: "Name of the infrastructure-as-code tool that owns this object, absent when a person created it.",
			},
			"team_id":     schema.StringAttribute{Computed: true},
			"kafka_topic": schema.StringAttribute{Computed: true},
			"heartbeat":   heartbeatDataSourceSchema(),
			"legacy_pool": legacyPoolDataSourceSchema(),
			"created_at":  schema.StringAttribute{Computed: true},
			"updated_at":  schema.StringAttribute{Computed: true},
		},
	}
}

func (d *IntegrationDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *IntegrationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config integrationDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var m map[string]any
	if !config.ID.IsNull() && !config.ID.IsUnknown() && config.ID.ValueString() != "" {
		result, err := d.client.get(ctx, "/api/v1/integrations/"+config.ID.ValueString())
		if err != nil {
			if isNotFound(err) {
				resp.Diagnostics.AddError("integration not found", fmt.Sprintf("no integration with id %q", config.ID.ValueString()))
				return
			}
			resp.Diagnostics.AddError("read integration failed", err.Error())
			return
		}
		m = result
	} else if !config.Name.IsNull() && !config.Name.IsUnknown() && config.Name.ValueString() != "" {
		items, err := d.client.listAll(ctx, "/api/v1/integrations")
		if err != nil {
			resp.Diagnostics.AddError("list integrations failed", err.Error())
			return
		}
		m = lookupByName(items, config.Name.ValueString())
		if m == nil {
			resp.Diagnostics.AddError("integration not found", fmt.Sprintf("no integration with name %q", config.Name.ValueString()))
			return
		}
	} else {
		resp.Diagnostics.AddError("missing lookup key", "provide either id or name")
		return
	}

	model := integrationModelFromAPI(m)
	resp.Diagnostics.Append(resp.State.Set(ctx, &integrationDataSourceModel{
		ID:                 model.ID,
		Name:               model.Name,
		Key:                model.Key,
		Type:               model.Type,
		SourceType:         model.SourceType,
		GroupBy:            model.GroupBy,
		Routes:             model.Routes,
		NotificationPolicy: model.NotificationPolicy,
		Templates:          model.Templates,
		Pipeline:           model.Pipeline,
		ProvisionedBy:      model.ProvisionedBy,
		TeamID:             model.TeamID,
		KafkaTopic:         model.KafkaTopic,
		Heartbeat:          model.Heartbeat,
		LegacyPool:         model.LegacyPool,
		CreatedAt:          model.CreatedAt,
		UpdatedAt:          model.UpdatedAt,
	})...)
}

func notificationPolicyDataSourceSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{Computed: true, Attributes: map[string]schema.Attribute{
		"channels":               schema.ListAttribute{Computed: true, ElementType: types.StringType},
		"batch_timeout_seconds":  schema.Int64Attribute{Computed: true},
		"batch_deadline_seconds": schema.Int64Attribute{Computed: true},
		"epic_threshold_count":   schema.Int64Attribute{Computed: true},
		"epic_threshold_seconds": schema.Int64Attribute{Computed: true},
		"emergency_user_id":      schema.StringAttribute{Computed: true},
		"epic_user_id":           schema.StringAttribute{Computed: true},
	}}
}

func heartbeatDataSourceSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{Computed: true, Attributes: map[string]schema.Attribute{
		"interval_seconds": schema.Int64Attribute{Computed: true},
		"grace_seconds":    schema.Int64Attribute{Computed: true},
	}}
}

func legacyPoolDataSourceSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{Computed: true, Attributes: map[string]schema.Attribute{
		"name":              schema.StringAttribute{Computed: true},
		"description":       schema.StringAttribute{Computed: true},
		"emergency_user_id": schema.StringAttribute{Computed: true},
		"duty_user_id":      schema.StringAttribute{Computed: true},
		"emails":            schema.ListAttribute{Computed: true, ElementType: types.StringType},
	}}
}
