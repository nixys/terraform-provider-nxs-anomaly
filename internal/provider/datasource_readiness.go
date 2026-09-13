package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &ReadinessDataSource{}

type ReadinessDataSource struct{ client *client }

func NewReadinessDataSource() datasource.DataSource { return &ReadinessDataSource{} }

type readinessDataSourceModel struct {
	ID                 types.String `tfsdk:"id"`
	CheckedAt          types.String `tfsdk:"checked_at"`
	Ready              types.Bool   `tfsdk:"ready"`
	ProductionReady    types.Bool   `tfsdk:"production_ready"`
	Blockers           types.Int64  `tfsdk:"blockers"`
	Warnings           types.Int64  `tfsdk:"warnings"`
	BlockerFingerprint types.String `tfsdk:"blocker_fingerprint"`
	Checks             types.List   `tfsdk:"checks"`
	Acknowledgement    types.Object `tfsdk:"acknowledgement"`
}

var readinessCheckAttrTypes = map[string]attr.Type{
	"key":      types.StringType,
	"title":    types.StringType,
	"severity": types.StringType,
	"detail":   types.StringType,
	"items":    types.ListType{ElemType: types.StringType},
}

var readinessAcknowledgementAttrTypes = map[string]attr.Type{
	"at":          types.StringType,
	"actor":       types.StringType,
	"actor_id":    types.StringType,
	"reason":      types.StringType,
	"fingerprint": types.StringType,
	"current":     types.BoolType,
}

func (d *ReadinessDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_readiness"
}

func (d *ReadinessDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Whether this installation can actually page someone. Read it after the " +
			"objects are in place to fail a pipeline that has left a hole in the alert path.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Always set to \"readiness\".",
			},
			"checked_at": schema.StringAttribute{Computed: true},
			"ready": schema.BoolAttribute{
				Computed:    true,
				Description: "Nothing is in the way.",
			},
			"production_ready": schema.BoolAttribute{
				Computed: true,
				Description: "The weaker claim the activation gate uses: either nothing is in the " +
					"way, or a named person accepted exactly the blockers listed here.",
			},
			"blockers": schema.Int64Attribute{Computed: true},
			"warnings": schema.Int64Attribute{Computed: true},
			"blocker_fingerprint": schema.StringAttribute{
				Computed:    true,
				Description: "Identifies the exact set of blockers; empty when there are none.",
			},
			"checks": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key":   schema.StringAttribute{Computed: true},
						"title": schema.StringAttribute{Computed: true},
						"severity": schema.StringAttribute{
							Computed: true,
							Description: "ok, warning or blocker. A check that could not run is " +
								"reported as a blocker, never as a pass.",
						},
						"detail": schema.StringAttribute{Computed: true},
						"items": schema.ListAttribute{
							Computed:    true,
							ElementType: types.StringType,
							Description: "The specific objects at fault.",
						},
					},
				},
			},
			"acknowledgement": schema.SingleNestedAttribute{
				Computed: true,
				Description: "Who accepted which blockers, and why. Null when nobody has. " +
					"current is false once the set of blockers has changed since.",
				Attributes: map[string]schema.Attribute{
					"at":          schema.StringAttribute{Computed: true},
					"actor":       schema.StringAttribute{Computed: true},
					"actor_id":    schema.StringAttribute{Computed: true},
					"reason":      schema.StringAttribute{Computed: true},
					"fingerprint": schema.StringAttribute{Computed: true},
					"current":     schema.BoolAttribute{Computed: true},
				},
			},
		},
	}
}

func (d *ReadinessDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureDataSourceClient(req, resp)
}

func (d *ReadinessDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	result, err := d.client.get(ctx, "/api/v1/readiness")
	if err != nil {
		resp.Diagnostics.AddError("read readiness report failed", err.Error())
		return
	}

	rawChecks := mapSliceFromMap(result, "checks")
	checkVals := make([]attr.Value, 0, len(rawChecks))
	for _, c := range rawChecks {
		obj, diags := types.ObjectValue(readinessCheckAttrTypes, map[string]attr.Value{
			"key":      types.StringValue(strFromMap(c, "key")),
			"title":    types.StringValue(strFromMap(c, "title")),
			"severity": types.StringValue(strFromMap(c, "severity")),
			"detail":   types.StringValue(strFromMap(c, "detail")),
			"items":    stringSliceToList(stringSliceFromMap(c, "items")),
		})
		resp.Diagnostics.Append(diags...)
		checkVals = append(checkVals, obj)
	}
	checks, diags := types.ListValue(types.ObjectType{AttrTypes: readinessCheckAttrTypes}, checkVals)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	acknowledgement := types.ObjectNull(readinessAcknowledgementAttrTypes)
	if am, ok := result["acknowledgement"].(map[string]any); ok && am != nil {
		acknowledgement, diags = types.ObjectValue(readinessAcknowledgementAttrTypes, map[string]attr.Value{
			"at":          types.StringValue(strFromMap(am, "at")),
			"actor":       types.StringValue(strFromMap(am, "actor")),
			"actor_id":    types.StringValue(strFromMap(am, "actor_id")),
			"reason":      types.StringValue(strFromMap(am, "reason")),
			"fingerprint": types.StringValue(strFromMap(am, "fingerprint")),
			"current":     types.BoolValue(boolFromMap(am, "current", false)),
		})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &readinessDataSourceModel{
		ID:                 types.StringValue("readiness"),
		CheckedAt:          types.StringValue(strFromMap(result, "checked_at")),
		Ready:              types.BoolValue(boolFromMap(result, "ready", false)),
		ProductionReady:    types.BoolValue(boolFromMap(result, "production_ready", false)),
		Blockers:           types.Int64Value(int64FromMap(result, "blockers", 0)),
		Warnings:           types.Int64Value(int64FromMap(result, "warnings", 0)),
		BlockerFingerprint: types.StringValue(strFromMap(result, "blocker_fingerprint")),
		Checks:             checks,
		Acknowledgement:    acknowledgement,
	})...)
}
