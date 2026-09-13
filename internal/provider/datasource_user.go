package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &UserDataSource{}

type UserDataSource struct{ client *client }

func NewUserDataSource() datasource.DataSource { return &UserDataSource{} }

type userDataSourceModel struct {
	ID                   types.String `tfsdk:"id"`
	Username             types.String `tfsdk:"username"`
	Name                 types.String `tfsdk:"name"`
	Email                types.String `tfsdk:"email"`
	Phone                types.String `tfsdk:"phone"`
	TelegramID           types.String `tfsdk:"telegram_id"`
	Timezone             types.String `tfsdk:"timezone"`
	Locale               types.String `tfsdk:"locale"`
	ProvisionedBy        types.String `tfsdk:"provisioned_by"`
	OnDuty               types.Bool   `tfsdk:"on_duty"`
	Priority             types.String `tfsdk:"priority"`
	Role                 types.String `tfsdk:"role"`
	NotificationTargets  types.List   `tfsdk:"notification_targets"`
	NotificationPolicies types.Object `tfsdk:"notification_policies"`
	CreatedAt            types.String `tfsdk:"created_at"`
	UpdatedAt            types.String `tfsdk:"updated_at"`
}

func (d *UserDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (d *UserDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Looks up a single nxs-anomaly user by ID or username.",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Optional: true, Computed: true, Description: "Object ID to look up. Supply either id or the name/username lookup attribute."},
			"username":    schema.StringAttribute{Optional: true, Computed: true},
			"name":        schema.StringAttribute{Computed: true},
			"email":       schema.StringAttribute{Computed: true},
			"phone":       schema.StringAttribute{Computed: true},
			"telegram_id": schema.StringAttribute{Computed: true},
			"timezone":    schema.StringAttribute{Computed: true},
			"locale": schema.StringAttribute{
				Computed:    true,
				Description: "UI language for this person. Empty means the UI follows their browser.",
			},
			"provisioned_by": schema.StringAttribute{
				Computed:    true,
				Description: "Name of the infrastructure-as-code tool that owns this object, absent when a person created it.",
			},
			"on_duty":  schema.BoolAttribute{Computed: true},
			"priority": schema.StringAttribute{Computed: true},
			"role":     schema.StringAttribute{Computed: true},
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
	}
}

func (d *UserDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *UserDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config userDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var m map[string]any
	if !config.ID.IsNull() && !config.ID.IsUnknown() && config.ID.ValueString() != "" {
		result, err := d.client.get(ctx, "/api/v1/users/"+config.ID.ValueString())
		if err != nil {
			if isNotFound(err) {
				resp.Diagnostics.AddError("user not found", fmt.Sprintf("no user with id %q", config.ID.ValueString()))
				return
			}
			resp.Diagnostics.AddError("read user failed", err.Error())
			return
		}
		m = result
	} else if !config.Username.IsNull() && !config.Username.IsUnknown() && config.Username.ValueString() != "" {
		items, err := d.client.listAll(ctx, "/api/v1/users")
		if err != nil {
			resp.Diagnostics.AddError("list users failed", err.Error())
			return
		}
		for _, u := range items {
			if strFromMap(u, "username") == config.Username.ValueString() {
				m = u
				break
			}
		}
		if m == nil {
			resp.Diagnostics.AddError("user not found", fmt.Sprintf("no user with username %q", config.Username.ValueString()))
			return
		}
	} else {
		resp.Diagnostics.AddError("missing lookup key", "provide either id or username")
		return
	}

	model := userModelFromAPI(ctx, m)
	state := userDataSourceModel{
		ID:                   model.ID,
		Username:             model.Username,
		Name:                 model.Name,
		Email:                model.Email,
		Phone:                model.Phone,
		TelegramID:           model.TelegramID,
		Timezone:             model.Timezone,
		Locale:               model.Locale,
		ProvisionedBy:        model.ProvisionedBy,
		OnDuty:               model.OnDuty,
		Priority:             model.Priority,
		Role:                 model.Role,
		NotificationTargets:  model.NotificationTargets,
		NotificationPolicies: model.NotificationPolicies,
		CreatedAt:            model.CreatedAt,
		UpdatedAt:            model.UpdatedAt,
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func userNotificationPoliciesDataSourceSchema() schema.SingleNestedAttribute {
	steps := schema.ListNestedAttribute{Computed: true, NestedObject: schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			"channel":      schema.StringAttribute{Computed: true},
			"target":       schema.StringAttribute{Computed: true},
			"wait_minutes": schema.Int64Attribute{Computed: true},
		},
	}}
	return schema.SingleNestedAttribute{Computed: true, Attributes: map[string]schema.Attribute{
		"default": steps, "important": steps,
	}}
}
