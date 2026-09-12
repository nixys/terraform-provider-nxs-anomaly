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

var _ datasource.DataSource = &UsersDataSource{}

type UsersDataSource struct{ client *client }

func NewUsersDataSource() datasource.DataSource { return &UsersDataSource{} }

type usersDataSourceModel struct {
	ID    types.String `tfsdk:"id"`
	Users types.List   `tfsdk:"users"`
}

var userItemAttrTypes = map[string]attr.Type{
	"id":          types.StringType,
	"name":        types.StringType,
	"username":    types.StringType,
	"email":       types.StringType,
	"phone":       types.StringType,
	"telegram_id": types.StringType,
	"timezone":    types.StringType,
	"on_duty":     types.BoolType,
	"priority":    types.StringType,
	"role":        types.StringType,
	"notification_targets": types.ListType{
		ElemType: types.ObjectType{AttrTypes: notificationTargetAttrTypes},
	},
	"notification_policies": types.ObjectType{AttrTypes: userNotificationPoliciesAttrTypes},
	"created_at":            types.StringType,
	"updated_at":            types.StringType,
}

func (d *UsersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_users"
}

func (d *UsersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Returns all users defined in nxs-anomaly.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Always set to \"users\".",
			},
			"users": schema.ListNestedAttribute{
				Computed:    true,
				Description: "List of all users.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":          schema.StringAttribute{Computed: true},
						"name":        schema.StringAttribute{Computed: true},
						"username":    schema.StringAttribute{Computed: true},
						"email":       schema.StringAttribute{Computed: true},
						"phone":       schema.StringAttribute{Computed: true},
						"telegram_id": schema.StringAttribute{Computed: true},
						"timezone":    schema.StringAttribute{Computed: true},
						"on_duty":     schema.BoolAttribute{Computed: true},
						"priority":    schema.StringAttribute{Computed: true},
						"role":        schema.StringAttribute{Computed: true},
						"notification_targets": schema.ListNestedAttribute{
							Computed: true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"channel": schema.StringAttribute{Computed: true},
									"target":  schema.StringAttribute{Computed: true},
								},
							},
						},
						"notification_policies": userNotificationPoliciesDataSourceSchema(),
						"created_at":            schema.StringAttribute{Computed: true},
						"updated_at":            schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *UsersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *UsersDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	items, err := d.client.listAll(ctx, "/api/v1/users")
	if err != nil {
		resp.Diagnostics.AddError("list users failed", err.Error())
		return
	}

	userVals := make([]attr.Value, 0, len(items))
	for _, m := range items {
		obj, diags := userItemFromAPIVal(m)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		userVals = append(userVals, obj)
	}

	userList, diags := types.ListValue(types.ObjectType{AttrTypes: userItemAttrTypes}, userVals)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &usersDataSourceModel{
		ID:    types.StringValue("users"),
		Users: userList,
	})...)
}

func userItemFromAPIVal(m map[string]any) (types.Object, diag.Diagnostics) {
	var allDiags diag.Diagnostics

	targetVals := make([]attr.Value, 0)
	for _, t := range mapSliceFromMap(m, "notification_targets") {
		obj, diags := types.ObjectValue(notificationTargetAttrTypes, map[string]attr.Value{
			"channel": types.StringValue(strFromMap(t, "channel")),
			"target":  types.StringValue(strFromMap(t, "target")),
		})
		allDiags.Append(diags...)
		if allDiags.HasError() {
			return types.ObjectUnknown(userItemAttrTypes), allDiags
		}
		targetVals = append(targetVals, obj)
	}
	targetList, diags := types.ListValue(
		types.ObjectType{AttrTypes: notificationTargetAttrTypes}, targetVals,
	)
	allDiags.Append(diags...)
	if allDiags.HasError() {
		return types.ObjectUnknown(userItemAttrTypes), allDiags
	}
	policies := userNotificationPoliciesFromAPI(m["notification_policies"])

	obj, diags := types.ObjectValue(userItemAttrTypes, map[string]attr.Value{
		"id":                    types.StringValue(strFromMap(m, "id")),
		"name":                  types.StringValue(strFromMap(m, "name")),
		"username":              types.StringValue(strFromMap(m, "username")),
		"email":                 types.StringValue(strFromMap(m, "email")),
		"phone":                 types.StringValue(strFromMap(m, "phone")),
		"telegram_id":           types.StringValue(strFromMap(m, "telegram_id")),
		"timezone":              types.StringValue(strFromMap(m, "timezone")),
		"on_duty":               types.BoolValue(boolFromMap(m, "on_duty", false)),
		"priority":              types.StringValue(strDefault(strFromMap(m, "priority"), "medium")),
		"role":                  types.StringValue(strFromMap(m, "role")),
		"notification_targets":  targetList,
		"notification_policies": policies,
		"created_at":            types.StringValue(strFromMap(m, "created_at")),
		"updated_at":            types.StringValue(strFromMap(m, "updated_at")),
	})
	allDiags.Append(diags...)
	return obj, allDiags
}
