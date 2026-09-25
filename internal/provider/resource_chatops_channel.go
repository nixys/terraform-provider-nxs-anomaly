package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &ChatopsChannelResource{}
var _ resource.ResourceWithImportState = &ChatopsChannelResource{}

type ChatopsChannelResource struct{ client *client }

func NewChatopsChannelResource() resource.Resource { return &ChatopsChannelResource{} }

func (r *ChatopsChannelResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_chatops_channel"
}

type chatopsChannelModel struct {
	ID                   types.String `tfsdk:"id"`
	Platform             types.String `tfsdk:"platform"`
	Name                 types.String `tfsdk:"name"`
	TeamID               types.String `tfsdk:"team_id"`
	UserID               types.String `tfsdk:"user_id"`
	CommandsEnabled      types.Bool   `tfsdk:"commands_enabled"`
	NotificationsEnabled types.Bool   `tfsdk:"notifications_enabled"`
	WebhookURL           types.String `tfsdk:"webhook_url"`
	Headers              types.Map    `tfsdk:"headers"`
	ExternalID           types.String `tfsdk:"external_id"`
	CreatedAt            types.String `tfsdk:"created_at"`
	ProvisionedBy        types.String `tfsdk:"provisioned_by"`
	UpdatedAt            types.String `tfsdk:"updated_at"`
}

func (r *ChatopsChannelResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a nxs-anomaly ChatOps channel.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"platform": schema.StringAttribute{
				Required:    true,
				Description: "ChatOps platform: telegram, slack, mattermost.",
				Validators:  []validator.String{stringvalidator.OneOf(validChatopsPlatforms...)},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Channel name or identifier on the platform.",
			},
			"team_id": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "ID of the team that owns this channel.",
			},
			"user_id": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "ID of the user that owns this channel (for direct channels).",
			},
			"commands_enabled": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
			"notifications_enabled": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
			"webhook_url": schema.StringAttribute{
				Optional: true, Computed: true, Sensitive: true,
				Description: "Incoming webhook URL or env: secret reference used for outbound ChatOps messages.",
			},
			"headers": schema.MapAttribute{
				Optional:    true,
				Sensitive:   true,
				ElementType: types.StringType,
				Description: outboundHeadersDescription,
				Validators:  outboundHeadersValidators(),
			},
			"external_id": schema.StringAttribute{
				Optional: true, Computed: true,
				Description: "Channel identifier used by the external chat platform.",
			},
			"created_at": schema.StringAttribute{Computed: true},
			"provisioned_by": schema.StringAttribute{
				Computed: true,
				Description: "Set to \"terraform\" for objects this provider created. The API then " +
					"refuses edits from anyone else and the web UI disables its own controls, so an " +
					"out-of-band change cannot be silently undone by the next apply.",
			},
			"updated_at": schema.StringAttribute{Computed: true},
		},
	}
}

func (r *ChatopsChannelResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client)
	if !ok {
		resp.Diagnostics.AddError("unexpected provider data type", fmt.Sprintf("got %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *ChatopsChannelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan chatopsChannelModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"platform":              plan.Platform.ValueString(),
		"name":                  plan.Name.ValueString(),
		"commands_enabled":      plan.CommandsEnabled.ValueBool(),
		"notifications_enabled": plan.NotificationsEnabled.ValueBool(),
	}
	if !plan.TeamID.IsNull() && !plan.TeamID.IsUnknown() && plan.TeamID.ValueString() != "" {
		body["team_id"] = plan.TeamID.ValueString()
	}
	if !plan.UserID.IsNull() && !plan.UserID.IsUnknown() && plan.UserID.ValueString() != "" {
		body["user_id"] = plan.UserID.ValueString()
	}
	if !plan.WebhookURL.IsNull() && !plan.WebhookURL.IsUnknown() {
		body["webhook_url"] = plan.WebhookURL.ValueString()
	}
	if !plan.ExternalID.IsNull() && !plan.ExternalID.IsUnknown() {
		body["external_id"] = plan.ExternalID.ValueString()
	}
	if h := headersFromAttr(plan.Headers); h != nil {
		body["headers"] = h
	}

	result, err := r.client.post(ctx, "/api/v1/chatops/channels", body)
	if err != nil {
		resp.Diagnostics.AddError("create chatops channel failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, chatopsChannelModelFromAPI(result))...)
}

func (r *ChatopsChannelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state chatopsChannelModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.get(ctx, "/api/v1/chatops/channels/"+state.ID.ValueString())
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read chatops channel failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, chatopsChannelModelFromAPI(result))...)
}

func (r *ChatopsChannelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan chatopsChannelModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"name":                  plan.Name.ValueString(),
		"platform":              plan.Platform.ValueString(),
		"commands_enabled":      plan.CommandsEnabled.ValueBool(),
		"notifications_enabled": plan.NotificationsEnabled.ValueBool(),
	}
	if !plan.TeamID.IsNull() && !plan.TeamID.IsUnknown() {
		body["team_id"] = plan.TeamID.ValueString()
	}
	if !plan.UserID.IsNull() && !plan.UserID.IsUnknown() {
		body["user_id"] = plan.UserID.ValueString()
	}
	if !plan.WebhookURL.IsNull() && !plan.WebhookURL.IsUnknown() {
		body["webhook_url"] = plan.WebhookURL.ValueString()
	}
	if !plan.ExternalID.IsNull() && !plan.ExternalID.IsUnknown() {
		body["external_id"] = plan.ExternalID.ValueString()
	}
	// The API merges an update, so a headers map removed from the configuration
	// has to be cleared explicitly: an empty object clears, an absent one keeps.
	if h := headersFromAttr(plan.Headers); h != nil {
		body["headers"] = h
	} else {
		body["headers"] = map[string]any{}
	}

	result, err := r.client.put(ctx, "/api/v1/chatops/channels/"+plan.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("update chatops channel failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, chatopsChannelModelFromAPI(result))...)
}

func (r *ChatopsChannelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state chatopsChannelModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.delete(ctx, "/api/v1/chatops/channels/"+state.ID.ValueString()); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete chatops channel failed", err.Error())
	}
}

func (r *ChatopsChannelResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func chatopsChannelModelFromAPI(m map[string]any) chatopsChannelModel {
	teamID := types.StringValue(strFromMap(m, "team_id"))
	if strFromMap(m, "team_id") == "" {
		teamID = types.StringNull()
	}
	userID := types.StringValue(strFromMap(m, "user_id"))
	if strFromMap(m, "user_id") == "" {
		userID = types.StringNull()
	}
	return chatopsChannelModel{
		ID:                   types.StringValue(strFromMap(m, "id")),
		Platform:             types.StringValue(strFromMap(m, "platform")),
		Name:                 types.StringValue(strFromMap(m, "name")),
		TeamID:               teamID,
		UserID:               userID,
		CommandsEnabled:      types.BoolValue(boolFromMap(m, "commands_enabled", true)),
		NotificationsEnabled: types.BoolValue(boolFromMap(m, "notifications_enabled", true)),
		WebhookURL:           nullableString(strFromMap(m, "webhook_url")),
		Headers:              headersToMap(m["headers"]),
		ExternalID:           nullableString(strFromMap(m, "external_id")),
		CreatedAt:            types.StringValue(strFromMap(m, "created_at")),
		ProvisionedBy:        nullableString(strFromMap(m, "provisioned_by")),
		UpdatedAt:            types.StringValue(strFromMap(m, "updated_at")),
	}
}
