package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lettermint/lettermint-go/v2"
)

var (
	_ resource.Resource                = (*webhookResource)(nil)
	_ resource.ResourceWithConfigure   = (*webhookResource)(nil)
	_ resource.ResourceWithImportState = (*webhookResource)(nil)
)

type webhookResource struct {
	client *clientData
}

type webhookResourceModel struct {
	ID                   types.String `tfsdk:"id"`
	RouteID              types.String `tfsdk:"route_id"`
	Name                 types.String `tfsdk:"name"`
	URL                  types.String `tfsdk:"url"`
	Events               types.List   `tfsdk:"events"`
	Enabled              types.Bool   `tfsdk:"enabled"`
	IncludeMachineEvents types.Bool   `tfsdk:"include_machine_events"`
	Secret               types.String `tfsdk:"secret"`
	LastCalledAt         types.String `tfsdk:"last_called_at"`
	CreatedAt            types.String `tfsdk:"created_at"`
	UpdatedAt            types.String `tfsdk:"updated_at"`
}

func NewWebhookResource() resource.Resource {
	return &webhookResource{}
}

func (r *webhookResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webhook"
}

func webhookResourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manage a route-scoped Lettermint webhook.",
		Attributes: map[string]schema.Attribute{
			"id":       computedIDAttribute(),
			"route_id": schema.StringAttribute{Required: true, Description: "Route ID. The current SDK supports route-scoped webhooks only.", Validators: uuidValidators(), PlanModifiers: requiresReplace()},
			"name":     schema.StringAttribute{Required: true, Description: "Webhook name.", Validators: []validator.String{stringvalidator.LengthAtMost(255)}},
			"url":      schema.StringAttribute{Required: true, Description: "Webhook URL.", Validators: append([]validator.String{stringvalidator.LengthAtMost(500)}, absoluteURIValidators()...)},
			"events": schema.ListAttribute{
				Required: true, Description: "Webhook events.", ElementType: types.StringType,
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
					listvalidator.ValueStringsAre(stringvalidator.OneOf(
						"message.created", "message.sent", "message.delivered", "message.auto_replied",
						"message.hard_bounced", "message.soft_bounced", "message.spam_complaint", "message.failed",
						"message.suppressed", "message.unsubscribed", "message.opened", "message.clicked",
						"message.inbound", "message.policy_rejected", "message.scheduled", "message.rescheduled",
						"message.canceled", "message.released", "suppression.added", "suppression.removed", "webhook.test",
					)),
				},
			},
			"enabled":                schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), Description: "Whether the webhook is enabled."},
			"include_machine_events": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Whether machine events are included."},
			"secret":                 schema.StringAttribute{Computed: true, Sensitive: true, Description: "Signing secret returned once when the webhook is created."},
			"last_called_at":         schema.StringAttribute{Computed: true, Description: "Last call time."},
			"created_at":             schema.StringAttribute{Computed: true, Description: "Creation time."},
			"updated_at":             schema.StringAttribute{Computed: true, Description: "Last update time."},
		},
	}
}

func (r *webhookResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = webhookResourceSchema()
}

func (r *webhookResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	requireClient(req.ProviderData, &r.client, &resp.Diagnostics)
}

func (r *webhookResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan webhookResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	events := webhookEvents(ctx, plan.Events, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.API.Webhooks.Create(ctx, lettermint.WebhookStoreRequest{
		RouteID:              plan.RouteID.ValueString(),
		Name:                 plan.Name.ValueString(),
		URL:                  plan.URL.ValueString(),
		Events:               events,
		Enabled:              boolPointer(plan.Enabled),
		IncludeMachineEvents: boolPointer(plan.IncludeMachineEvents),
	})
	if err != nil {
		appendClientDiagnostic(&resp.Diagnostics, "Cannot create webhook", err)
		return
	}
	state := webhookModelFromAPI(ctx, result.Data, plan, &resp.Diagnostics)
	state.Secret = types.StringValue(result.Data.Secret)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *webhookResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state webhookResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := r.client.API.Webhooks.Retrieve(ctx, state.ID.ValueString())
	if isNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		appendClientDiagnostic(&resp.Diagnostics, "Cannot read webhook", err)
		return
	}
	state = webhookModelFromAPI(ctx, lettermint.WebhookData(result), state, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *webhookResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan webhookResourceModel
	var state webhookResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	events := webhookEvents(ctx, plan.Events, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := r.client.API.Webhooks.Update(ctx, state.ID.ValueString(), lettermint.WebhookUpdateRequest{
		Name:                 plan.Name.ValueString(),
		URL:                  plan.URL.ValueString(),
		Events:               events,
		Enabled:              boolPointer(plan.Enabled),
		IncludeMachineEvents: boolPointer(plan.IncludeMachineEvents),
	})
	if err != nil {
		appendClientDiagnostic(&resp.Diagnostics, "Cannot update webhook", err)
		return
	}
	plan.Secret = state.Secret
	plan = webhookModelFromAPI(ctx, result.Data, plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *webhookResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state webhookResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, err := r.client.API.Webhooks.Delete(ctx, state.ID.ValueString())
	if err != nil && !isNotFound(err) {
		appendClientDiagnostic(&resp.Diagnostics, "Cannot delete webhook", err)
	}
}

func (r *webhookResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func webhookEvents(ctx context.Context, value types.List, diags *diag.Diagnostics) []lettermint.APIWebhookEvent {
	var values []string
	diags.Append(value.ElementsAs(ctx, &values, false)...)
	events := make([]lettermint.APIWebhookEvent, 0, len(values))
	for _, value := range values {
		events = append(events, lettermint.APIWebhookEvent(value))
	}
	return events
}

func webhookModelFromAPI(ctx context.Context, webhook lettermint.WebhookData, previous webhookResourceModel, diags *diag.Diagnostics) webhookResourceModel {
	previous.ID = stringValue(webhook.ID)
	previous.RouteID = stringValue(webhook.RouteID)
	previous.Name = stringValue(webhook.Name)
	previous.URL = stringValue(webhook.URL)
	elements := make([]attr.Value, 0, len(webhook.Events))
	for _, event := range webhook.Events {
		elements = append(elements, types.StringValue(event))
	}
	previous.Events = types.ListValueMust(types.StringType, elements)
	previous.Enabled = types.BoolValue(webhook.Enabled)
	previous.IncludeMachineEvents = types.BoolValue(webhook.IncludeMachineEvents)
	if webhook.Secret != "" {
		previous.Secret = types.StringValue(webhook.Secret)
	}
	previous.LastCalledAt = nullableString(webhook.LastCalledAt)
	previous.CreatedAt = stringValue(webhook.CreatedAt)
	previous.UpdatedAt = stringValue(webhook.UpdatedAt)
	return previous
}

func webhookDataSourceModelFromAPI(webhook lettermint.WebhookData) webhookDataSourceModel {
	elements := make([]attr.Value, 0, len(webhook.Events))
	for _, event := range webhook.Events {
		elements = append(elements, types.StringValue(event))
	}
	return webhookDataSourceModel{
		ID:                   stringValue(webhook.ID),
		RouteID:              stringValue(webhook.RouteID),
		Name:                 stringValue(webhook.Name),
		URL:                  stringValue(webhook.URL),
		Events:               types.ListValueMust(types.StringType, elements),
		Enabled:              types.BoolValue(webhook.Enabled),
		IncludeMachineEvents: types.BoolValue(webhook.IncludeMachineEvents),
		LastCalledAt:         nullableString(webhook.LastCalledAt),
		CreatedAt:            stringValue(webhook.CreatedAt),
		UpdatedAt:            stringValue(webhook.UpdatedAt),
	}
}
