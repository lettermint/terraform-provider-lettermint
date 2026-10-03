package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/float64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lettermint/lettermint-go/v3"
)

var (
	_ resource.Resource                = (*routeResource)(nil)
	_ resource.ResourceWithConfigure   = (*routeResource)(nil)
	_ resource.ResourceWithImportState = (*routeResource)(nil)
)

type routeResource struct {
	client *clientData
}

type routeResourceModel struct {
	ID                           types.String  `tfsdk:"id"`
	ProjectID                    types.String  `tfsdk:"project_id"`
	Slug                         types.String  `tfsdk:"slug"`
	Name                         types.String  `tfsdk:"name"`
	RouteType                    types.String  `tfsdk:"route_type"`
	IsDefault                    types.Bool    `tfsdk:"is_default"`
	InboundAddress               types.String  `tfsdk:"inbound_address"`
	InboundDomain                types.String  `tfsdk:"inbound_domain"`
	InboundDomainVerifiedAt      types.String  `tfsdk:"inbound_domain_verified_at"`
	InboundSpamThreshold         types.Float64 `tfsdk:"inbound_spam_threshold"`
	AttachmentDelivery           types.String  `tfsdk:"attachment_delivery"`
	TrackOpens                   types.Bool    `tfsdk:"track_opens"`
	TrackClicks                  types.Bool    `tfsdk:"track_clicks"`
	GeneratePlaintextFallback    types.Bool    `tfsdk:"generate_plaintext_fallback"`
	SuppressAutoResponders       types.Bool    `tfsdk:"suppress_auto_responders"`
	SuppressDisposableRecipients types.Bool    `tfsdk:"suppress_disposable_recipients"`
	TLS                          types.String  `tfsdk:"tls"`
	DisableHostedUnsubscribe     types.Bool    `tfsdk:"disable_hosted_unsubscribe"`
	RedactEmailContent           types.Bool    `tfsdk:"redact_email_content"`
	CreatedAt                    types.String  `tfsdk:"created_at"`
	UpdatedAt                    types.String  `tfsdk:"updated_at"`
}

func NewRouteResource() resource.Resource {
	return &routeResource{}
}

func (r *routeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_route"
}

func (r *routeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage a Lettermint route.",
		Attributes: map[string]schema.Attribute{
			"id":         computedIDAttribute(),
			"project_id": schema.StringAttribute{Required: true, Description: "Project ID.", PlanModifiers: requiresReplace()},
			"slug": schema.StringAttribute{
				Optional: true, Computed: true, Description: "Route slug.",
				Validators: []validator.String{stringvalidator.LengthAtMost(255)}, PlanModifiers: requiresReplace(),
			},
			"name": schema.StringAttribute{Required: true, Description: "Route name.", Validators: []validator.String{stringvalidator.LengthAtMost(255)}},
			"route_type": schema.StringAttribute{
				Required: true, Description: "Route type.", Validators: enum("transactional", "broadcast", "inbound"), PlanModifiers: requiresReplace(),
			},
			"is_default":                 schema.BoolAttribute{Computed: true, Description: "Whether this is the default route."},
			"inbound_address":            schema.StringAttribute{Computed: true, Description: "Generated inbound address."},
			"inbound_domain":             schema.StringAttribute{Optional: true, Computed: true, Description: "Custom inbound domain.", Validators: []validator.String{stringvalidator.LengthAtMost(255)}},
			"inbound_domain_verified_at": schema.StringAttribute{Computed: true, Description: "Inbound domain verification time."},
			"inbound_spam_threshold": schema.Float64Attribute{
				Optional: true, Computed: true, Description: "Inbound spam threshold.",
				Validators: []validator.Float64{float64validator.Between(0, 10)},
			},
			"attachment_delivery":            schema.StringAttribute{Optional: true, Computed: true, Description: "Inbound attachment delivery mode.", Validators: enum("inline", "url")},
			"track_opens":                    schema.BoolAttribute{Optional: true, Computed: true, Description: "Whether open tracking is enabled."},
			"track_clicks":                   schema.BoolAttribute{Optional: true, Computed: true, Description: "Whether click tracking is enabled."},
			"generate_plaintext_fallback":    schema.BoolAttribute{Optional: true, Computed: true, Description: "Whether Lettermint generates a plaintext fallback."},
			"suppress_auto_responders":       schema.BoolAttribute{Optional: true, Computed: true, Description: "Whether outbound messages suppress auto responders."},
			"suppress_disposable_recipients": schema.BoolAttribute{Optional: true, Computed: true, Description: "Whether disposable recipients are suppressed for each message."},
			"tls":                            schema.StringAttribute{Optional: true, Computed: true, Description: "TLS delivery policy.", Validators: enum("opportunistic", "enforced")},
			"disable_hosted_unsubscribe":     schema.BoolAttribute{Optional: true, Computed: true, Description: "Whether hosted unsubscribe injection is disabled."},
			"redact_email_content":           schema.BoolAttribute{Optional: true, Computed: true, Description: "Whether dashboard email content is redacted."},
			"created_at":                     schema.StringAttribute{Computed: true, Description: "Creation time."},
			"updated_at":                     schema.StringAttribute{Computed: true, Description: "Last update time."},
		},
	}
}

func (r *routeResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	requireClient(req.ProviderData, &r.client, &resp.Diagnostics)
}

func (r *routeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan routeResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.Routes.Create(ctx, plan.ProjectID.ValueString(), lettermint.StoreRouteData{
		Name:      plan.Name.ValueString(),
		RouteType: lettermint.RouteType(plan.RouteType.ValueString()),
		Slug:      knownString(plan.Slug),
	})
	if err != nil {
		appendClientDiagnostic(&resp.Diagnostics, "Cannot create route", err)
		return
	}

	remote := created.Data
	update := routeUpdateRequest(plan)
	if update.Settings.IsSet() || update.InboundSettings.IsSet() {
		updated, updateErr := r.client.Routes.Update(ctx, remote.ID, update)
		if updateErr != nil {
			appendClientDiagnostic(&resp.Diagnostics, "Route was created, but its settings could not be updated", updateErr)
			return
		}
		remote = updated.Data
	}

	state := routeModelFromAPI(remote)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *routeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state routeResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := r.client.Routes.Retrieve(ctx, state.ID.ValueString(), nil)
	if isNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		appendClientDiagnostic(&resp.Diagnostics, "Cannot read route", err)
		return
	}
	state = routeModelFromAPI(*result)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *routeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan routeResourceModel
	var state routeResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	update := routeUpdateRequest(plan)
	update.Name = knownString(plan.Name)
	result, err := r.client.Routes.Update(ctx, state.ID.ValueString(), update)
	if err != nil {
		appendClientDiagnostic(&resp.Diagnostics, "Cannot update route", err)
		return
	}
	plan = routeModelFromAPI(result.Data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *routeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state routeResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, err := r.client.Routes.Delete(ctx, state.ID.ValueString())
	if err != nil && !isNotFound(err) {
		appendClientDiagnostic(&resp.Diagnostics, "Cannot delete route", err)
	}
}

func (r *routeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// routeUpdateRequest maps the planned route settings to an update request.
// Known values are sent. Null or unknown values are absent, so the API keeps
// them. A settings object without a known value is absent too.
func routeUpdateRequest(model routeResourceModel) lettermint.UpdateRouteData {
	var request lettermint.UpdateRouteData

	settings := lettermint.UpdateRouteSettingsData{
		TrackOpens:                   knownBool(model.TrackOpens),
		TrackClicks:                  knownBool(model.TrackClicks),
		GeneratePlaintextFallback:    knownBool(model.GeneratePlaintextFallback),
		SuppressAutoResponders:       knownBool(model.SuppressAutoResponders),
		SuppressDisposableRecipients: knownBool(model.SuppressDisposableRecipients),
		TLS:                          knownEnum[lettermint.TlsPolicy](model.TLS),
		DisableHostedUnsubscribe:     knownBool(model.DisableHostedUnsubscribe),
		RedactEmailContent:           knownBool(model.RedactEmailContent),
	}
	if settings.TrackOpens.IsSet() || settings.TrackClicks.IsSet() || settings.GeneratePlaintextFallback.IsSet() || settings.SuppressAutoResponders.IsSet() || settings.SuppressDisposableRecipients.IsSet() || settings.TLS.IsSet() || settings.DisableHostedUnsubscribe.IsSet() || settings.RedactEmailContent.IsSet() {
		request.Settings = lettermint.Value(settings)
	}

	inbound := lettermint.UpdateRouteInboundSettingsData{
		InboundDomain:        knownString(model.InboundDomain),
		InboundSpamThreshold: knownFloat64(model.InboundSpamThreshold),
		AttachmentDelivery:   knownEnum[lettermint.AttachmentDelivery](model.AttachmentDelivery),
	}
	if inbound.InboundDomain.IsSet() || inbound.InboundSpamThreshold.IsSet() || inbound.AttachmentDelivery.IsSet() {
		request.InboundSettings = lettermint.Value(inbound)
	}

	return request
}

func routeModelFromAPI(route lettermint.RouteData) routeResourceModel {
	model := routeResourceModel{
		ID:                      stringValue(route.ID),
		ProjectID:               stringValue(route.ProjectID),
		Slug:                    stringValue(route.Slug),
		Name:                    stringValue(route.Name),
		RouteType:               stringValue(string(route.RouteType)),
		IsDefault:               types.BoolValue(route.IsDefault),
		InboundAddress:          optionalString(route.InboundAddress),
		InboundDomain:           optionalString(route.InboundDomain),
		InboundDomainVerifiedAt: optionalString(route.InboundDomainVerifiedAt),
		InboundSpamThreshold:    optionalFloat64(route.InboundSpamThreshold),
		CreatedAt:               stringValue(route.CreatedAt),
		UpdatedAt:               stringValue(route.UpdatedAt),
	}
	if route.AttachmentDelivery == nil || *route.AttachmentDelivery == "" {
		model.AttachmentDelivery = types.StringNull()
	} else {
		model.AttachmentDelivery = stringValue(string(*route.AttachmentDelivery))
	}
	settings, _ := route.Settings.Get()
	model.TrackOpens = optionalBool(settings.TrackOpens)
	model.TrackClicks = optionalBool(settings.TrackClicks)
	model.GeneratePlaintextFallback = optionalBool(settings.GeneratePlaintextFallback)
	model.SuppressAutoResponders = optionalBool(settings.SuppressAutoResponders)
	model.SuppressDisposableRecipients = optionalBool(settings.SuppressDisposableRecipients)
	model.DisableHostedUnsubscribe = optionalBool(settings.DisableHostedUnsubscribe)
	model.RedactEmailContent = optionalBool(settings.RedactEmailContent)
	if settings.TLS == nil {
		model.TLS = types.StringNull()
	} else {
		model.TLS = stringValue(string(*settings.TLS))
	}
	return model
}
