package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &UserResource{}
var _ resource.ResourceWithImportState = &UserResource{}

type UserResource struct{ client *client }

func NewUserResource() resource.Resource { return &UserResource{} }

func (r *UserResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

type userModel struct {
	ID                   types.String `tfsdk:"id"`
	Name                 types.String `tfsdk:"name"`
	Username             types.String `tfsdk:"username"`
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

var notificationTargetAttrTypes = map[string]attr.Type{
	"channel": types.StringType,
	"target":  types.StringType,
}

var notificationPolicyStepAttrTypes = map[string]attr.Type{
	"channel":      types.StringType,
	"target":       types.StringType,
	"wait_minutes": types.Int64Type,
}

var userNotificationPoliciesAttrTypes = map[string]attr.Type{
	"default":   types.ListType{ElemType: types.ObjectType{AttrTypes: notificationPolicyStepAttrTypes}},
	"important": types.ListType{ElemType: types.ObjectType{AttrTypes: notificationPolicyStepAttrTypes}},
}

func (r *UserResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a nxs-anomaly user.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Full display name of the user.",
			},
			"username": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Login username. Defaults to lower-cased name with spaces replaced by dots.",
			},
			"email": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(""),
			},
			"phone": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(""),
			},
			"telegram_id": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(""),
			},
			"timezone": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("UTC"),
				Description: "IANA timezone name.",
			},
			"locale": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(""),
				Validators: []validator.String{
					stringvalidator.OneOf("", "ru-RU", "en-US"),
				},
				Description: "UI language for this person. Empty means the UI follows their browser.",
			},
			"provisioned_by": schema.StringAttribute{
				Computed: true,
				Description: "Set to \"terraform\" for objects this provider created. The API then " +
					"refuses edits from anyone else and the web UI disables its own controls, so an " +
					"out-of-band change cannot be silently undone by the next apply.",
			},
			"on_duty": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"priority": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("medium"),
				Description: "Notification priority: low, medium, high.",
				Validators:  []validator.String{stringvalidator.OneOf(validPriorities...)},
			},
			"role": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
				Description: "Login role: viewer, responder, editor, admin; empty disables login permissions.",
				Validators:  []validator.String{stringvalidator.OneOf(validUserRoles...)},
			},
			"notification_targets": schema.ListNestedAttribute{
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
				Optional:      true,
				Computed:      true,
				Description:   "List of notification targets for this user.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"channel": schema.StringAttribute{
							Required:    true,
							Description: "Channel type (webhook, telegram, email, sms, phone).",
						},
						"target": schema.StringAttribute{
							Required:    true,
							Description: "Channel-specific target address (URL, chat ID, email, phone number).",
						},
					},
				},
			},
			"notification_policies": schema.SingleNestedAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Personal notification policies for default and important alerts.",
				Attributes: map[string]schema.Attribute{
					"default":   notificationPolicyStepsSchema(),
					"important": notificationPolicyStepsSchema(),
				},
			},
			"created_at": schema.StringAttribute{Computed: true},
			"updated_at": schema.StringAttribute{Computed: true},
		},
	}
}

func (r *UserResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *UserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan userModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := userBody(plan)

	result, err := r.client.post(ctx, "/api/v1/users", body)
	if err != nil {
		resp.Diagnostics.AddError("create user failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, userModelFromAPI(ctx, result))...)
}

func (r *UserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state userModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.get(ctx, "/api/v1/users/"+state.ID.ValueString())
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("read user failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, userModelFromAPI(ctx, result))...)
}

func (r *UserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan userModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := userBody(plan)

	result, err := r.client.put(ctx, "/api/v1/users/"+plan.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("update user failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, userModelFromAPI(ctx, result))...)
}

func (r *UserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state userModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.delete(ctx, "/api/v1/users/"+state.ID.ValueString()); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("delete user failed", err.Error())
	}
}

func (r *UserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func userModelFromAPI(ctx context.Context, m map[string]any) userModel {
	var targets []attr.Value
	for _, t := range mapSliceFromMap(m, "notification_targets") {
		channel := strFromMap(t, "type")
		if channel == "" {
			channel = strFromMap(t, "channel")
		}
		obj, _ := types.ObjectValue(notificationTargetAttrTypes, map[string]attr.Value{
			"channel": types.StringValue(channel),
			"target":  types.StringValue(strFromMap(t, "target")),
		})
		targets = append(targets, obj)
	}
	targetList, _ := types.ListValue(types.ObjectType{AttrTypes: notificationTargetAttrTypes}, targets)
	policies := userNotificationPoliciesFromAPI(m["notification_policies"])

	return userModel{
		ID:                   types.StringValue(strFromMap(m, "id")),
		Name:                 types.StringValue(strFromMap(m, "name")),
		Username:             types.StringValue(strFromMap(m, "username")),
		Email:                types.StringValue(strFromMap(m, "email")),
		Phone:                types.StringValue(strFromMap(m, "phone")),
		TelegramID:           types.StringValue(strFromMap(m, "telegram_id")),
		Timezone:             types.StringValue(strFromMap(m, "timezone")),
		Locale:               types.StringValue(strFromMap(m, "locale")),
		ProvisionedBy:        nullableString(strFromMap(m, "provisioned_by")),
		OnDuty:               types.BoolValue(boolFromMap(m, "on_duty", false)),
		Priority:             types.StringValue(strDefault(strFromMap(m, "priority"), "medium")),
		Role:                 types.StringValue(strFromMap(m, "role")),
		NotificationTargets:  targetList,
		NotificationPolicies: policies,
		CreatedAt:            types.StringValue(strFromMap(m, "created_at")),
		UpdatedAt:            types.StringValue(strFromMap(m, "updated_at")),
	}
}

func notificationPolicyStepsSchema() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		Optional: true,
		Computed: true,
		NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"channel": schema.StringAttribute{Required: true},
			"target": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString(""),
			},
			"wait_minutes": schema.Int64Attribute{
				Optional: true, Computed: true, Default: int64default.StaticInt64(0),
			},
		}},
	}
}

func userBody(plan userModel) map[string]any {
	body := map[string]any{
		"name":                 plan.Name.ValueString(),
		"email":                plan.Email.ValueString(),
		"phone":                plan.Phone.ValueString(),
		"telegram_id":          plan.TelegramID.ValueString(),
		"timezone":             plan.Timezone.ValueString(),
		"locale":               plan.Locale.ValueString(),
		"on_duty":              plan.OnDuty.ValueBool(),
		"priority":             plan.Priority.ValueString(),
		"role":                 plan.Role.ValueString(),
		"notification_targets": notificationTargetsFromPlan(plan.NotificationTargets),
	}
	if !plan.Username.IsNull() && !plan.Username.IsUnknown() {
		body["username"] = plan.Username.ValueString()
	}
	if !plan.NotificationPolicies.IsNull() && !plan.NotificationPolicies.IsUnknown() {
		body["notification_policies"] = notificationPoliciesFromPlan(plan.NotificationPolicies)
	}
	return body
}

func notificationPoliciesFromPlan(obj types.Object) map[string]any {
	if obj.IsNull() || obj.IsUnknown() {
		return nil
	}
	out := map[string]any{}
	for _, name := range []string{"default", "important"} {
		list, ok := obj.Attributes()[name].(types.List)
		if !ok || list.IsNull() || list.IsUnknown() {
			continue
		}
		steps := make([]any, 0, len(list.Elements()))
		for _, elem := range list.Elements() {
			step, ok := elem.(types.Object)
			if !ok {
				continue
			}
			a := step.Attributes()
			steps = append(steps, map[string]any{
				"channel": attrStr(a, "channel"), "target": attrStr(a, "target"),
				"wait_minutes": attrInt64(a, "wait_minutes", 0),
			})
		}
		out[name] = steps
	}
	return out
}

func userNotificationPoliciesFromAPI(raw any) types.Object {
	m, ok := raw.(map[string]any)
	if !ok || m == nil {
		return types.ObjectNull(userNotificationPoliciesAttrTypes)
	}
	values := map[string]attr.Value{}
	for _, name := range []string{"default", "important"} {
		steps := make([]attr.Value, 0)
		for _, s := range mapSliceFromMap(m, name) {
			obj, _ := types.ObjectValue(notificationPolicyStepAttrTypes, map[string]attr.Value{
				"channel":      types.StringValue(strFromMap(s, "channel")),
				"target":       types.StringValue(strFromMap(s, "target")),
				"wait_minutes": types.Int64Value(int64FromMap(s, "wait_minutes", 0)),
			})
			steps = append(steps, obj)
		}
		list, _ := types.ListValue(types.ObjectType{AttrTypes: notificationPolicyStepAttrTypes}, steps)
		values[name] = list
	}
	obj, _ := types.ObjectValue(userNotificationPoliciesAttrTypes, values)
	return obj
}

func notificationTargetsFromPlan(list types.List) []map[string]any {
	if list.IsNull() || list.IsUnknown() {
		return nil
	}
	var targets []map[string]any
	for _, elem := range list.Elements() {
		obj, ok := elem.(types.Object)
		if !ok {
			continue
		}
		attrs := obj.Attributes()
		channel, _ := attrs["channel"].(types.String)
		target, _ := attrs["target"].(types.String)
		targets = append(targets, map[string]any{
			"type":   channel.ValueString(),
			"target": target.ValueString(),
		})
	}
	return targets
}
