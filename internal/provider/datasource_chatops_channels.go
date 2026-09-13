package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &ChatopsChannelDataSource{}
var _ datasource.DataSource = &ChatopsChannelsDataSource{}

type ChatopsChannelDataSource struct{ client *client }
type ChatopsChannelsDataSource struct{ client *client }

func NewChatopsChannelDataSource() datasource.DataSource  { return &ChatopsChannelDataSource{} }
func NewChatopsChannelsDataSource() datasource.DataSource { return &ChatopsChannelsDataSource{} }

type chatopsChannelDataSourceModel struct {
	ID                   types.String `tfsdk:"id"`
	Platform             types.String `tfsdk:"platform"`
	Name                 types.String `tfsdk:"name"`
	TeamID               types.String `tfsdk:"team_id"`
	UserID               types.String `tfsdk:"user_id"`
	CommandsEnabled      types.Bool   `tfsdk:"commands_enabled"`
	NotificationsEnabled types.Bool   `tfsdk:"notifications_enabled"`
	WebhookURL           types.String `tfsdk:"webhook_url"`
	ExternalID           types.String `tfsdk:"external_id"`
	CreatedAt            types.String `tfsdk:"created_at"`
	UpdatedAt            types.String `tfsdk:"updated_at"`
}

type chatopsChannelsDataSourceModel struct {
	ID       types.String `tfsdk:"id"`
	Channels types.List   `tfsdk:"channels"`
}

var chatopsChannelItemAttrTypes = map[string]attr.Type{
	"id": types.StringType, "platform": types.StringType, "name": types.StringType,
	"team_id": types.StringType, "user_id": types.StringType,
	"commands_enabled": types.BoolType, "notifications_enabled": types.BoolType,
	"webhook_url": types.StringType, "external_id": types.StringType,
	"created_at": types.StringType, "updated_at": types.StringType,
}

func (d *ChatopsChannelDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_chatops_channel"
}

func (d *ChatopsChannelsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_chatops_channels"
}

func chatopsChannelComputedAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{Computed: true}, "platform": schema.StringAttribute{Computed: true},
		"name": schema.StringAttribute{Computed: true}, "team_id": schema.StringAttribute{Computed: true},
		"user_id": schema.StringAttribute{Computed: true}, "commands_enabled": schema.BoolAttribute{Computed: true},
		"notifications_enabled": schema.BoolAttribute{Computed: true},
		"webhook_url":           schema.StringAttribute{Computed: true, Sensitive: true},
		"external_id":           schema.StringAttribute{Computed: true}, "created_at": schema.StringAttribute{Computed: true},
		"updated_at": schema.StringAttribute{Computed: true},
	}
}

func (d *ChatopsChannelDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := chatopsChannelComputedAttributes()
	attrs["id"] = schema.StringAttribute{Optional: true, Computed: true}
	attrs["name"] = schema.StringAttribute{Optional: true, Computed: true}
	resp.Schema = schema.Schema{Description: "Looks up a ChatOps channel by ID or name.", Attributes: attrs}
}

func (d *ChatopsChannelsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{Description: "Returns all ChatOps channels.", Attributes: map[string]schema.Attribute{
		"id":       schema.StringAttribute{Computed: true},
		"channels": schema.ListNestedAttribute{Computed: true, NestedObject: schema.NestedAttributeObject{Attributes: chatopsChannelComputedAttributes()}},
	}}
}

func (d *ChatopsChannelDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ChatopsChannelsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ChatopsChannelDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config chatopsChannelDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var item map[string]any
	if !config.ID.IsNull() && !config.ID.IsUnknown() && config.ID.ValueString() != "" {
		result, err := d.client.get(ctx, "/api/v1/chatops/channels/"+config.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("read ChatOps channel failed", err.Error())
			return
		}
		item = result
	} else if !config.Name.IsNull() && !config.Name.IsUnknown() && config.Name.ValueString() != "" {
		items, err := d.client.listAll(ctx, "/api/v1/chatops/channels")
		if err != nil {
			resp.Diagnostics.AddError("list ChatOps channels failed", err.Error())
			return
		}
		item = lookupByName(items, config.Name.ValueString())
		if item == nil {
			resp.Diagnostics.AddError("ChatOps channel not found", fmt.Sprintf("no channel with name %q", config.Name.ValueString()))
			return
		}
	} else {
		resp.Diagnostics.AddError("missing lookup key", "provide either id or name")
		return
	}
	model := chatopsChannelModelFromAPI(item)
	resp.Diagnostics.Append(resp.State.Set(ctx, &chatopsChannelDataSourceModel{ID: model.ID, Platform: model.Platform, Name: model.Name, TeamID: model.TeamID, UserID: model.UserID, CommandsEnabled: model.CommandsEnabled, NotificationsEnabled: model.NotificationsEnabled, WebhookURL: model.WebhookURL, ExternalID: model.ExternalID, CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt})...)
}

func (d *ChatopsChannelsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	items, err := d.client.listAll(ctx, "/api/v1/chatops/channels")
	if err != nil {
		resp.Diagnostics.AddError("list ChatOps channels failed", err.Error())
		return
	}
	values := make([]attr.Value, 0, len(items))
	for _, item := range items {
		values = append(values, chatopsChannelItemFromAPI(item))
	}
	list, diags := types.ListValue(types.ObjectType{AttrTypes: chatopsChannelItemAttrTypes}, values)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &chatopsChannelsDataSourceModel{ID: types.StringValue("chatops_channels"), Channels: list})...)
}

func chatopsChannelItemFromAPI(item map[string]any) types.Object {
	m := chatopsChannelModelFromAPI(item)
	obj, _ := types.ObjectValue(chatopsChannelItemAttrTypes, map[string]attr.Value{"id": m.ID, "platform": m.Platform, "name": m.Name, "team_id": m.TeamID, "user_id": m.UserID, "commands_enabled": m.CommandsEnabled, "notifications_enabled": m.NotificationsEnabled, "webhook_url": m.WebhookURL, "external_id": m.ExternalID, "created_at": m.CreatedAt, "updated_at": m.UpdatedAt})
	return obj
}
