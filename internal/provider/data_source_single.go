package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type teamDataSource struct{ client *clientData }
type projectDataSource struct{ client *clientData }
type domainDataSource struct{ client *clientData }
type routeDataSource struct{ client *clientData }
type webhookDataSource struct{ client *clientData }

type teamDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Type           types.String `tfsdk:"type"`
	Plan           types.String `tfsdk:"plan"`
	IncludedVolume types.Int64  `tfsdk:"included_volume"`
	Tier           types.Int64  `tfsdk:"tier"`
	VerifiedAt     types.String `tfsdk:"verified_at"`
	CreatedAt      types.String `tfsdk:"created_at"`
}

type projectDataSourceModel struct {
	ID                 types.String `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	SMTPEnabled        types.Bool   `tfsdk:"smtp_enabled"`
	RedactEmailContent types.Bool   `tfsdk:"redact_email_content"`
	DefaultRouteID     types.String `tfsdk:"default_route_id"`
	TokenGeneratedAt   types.String `tfsdk:"token_generated_at"`
	TokenLastUsedAt    types.String `tfsdk:"token_last_used_at"`
	TokenLastUsedIP    types.String `tfsdk:"token_last_used_ip"`
	CreatedAt          types.String `tfsdk:"created_at"`
	UpdatedAt          types.String `tfsdk:"updated_at"`
}

type domainDataSourceModel struct {
	ID              types.String `tfsdk:"id"`
	Domain          types.String `tfsdk:"domain"`
	DKIMMode        types.String `tfsdk:"dkim_mode"`
	RotationReady   types.Bool   `tfsdk:"rotation_ready"`
	StatusChangedAt types.String `tfsdk:"status_changed_at"`
	CreatedAt       types.String `tfsdk:"created_at"`
}

type webhookDataSourceModel struct {
	ID                   types.String `tfsdk:"id"`
	RouteID              types.String `tfsdk:"route_id"`
	Name                 types.String `tfsdk:"name"`
	URL                  types.String `tfsdk:"url"`
	Events               types.List   `tfsdk:"events"`
	Enabled              types.Bool   `tfsdk:"enabled"`
	IncludeMachineEvents types.Bool   `tfsdk:"include_machine_events"`
	HasBasicAuth         types.Bool   `tfsdk:"has_basic_auth"`
	LastCalledAt         types.String `tfsdk:"last_called_at"`
	CreatedAt            types.String `tfsdk:"created_at"`
	UpdatedAt            types.String `tfsdk:"updated_at"`
}

func NewTeamDataSource() datasource.DataSource    { return &teamDataSource{} }
func NewProjectDataSource() datasource.DataSource { return &projectDataSource{} }
func NewDomainDataSource() datasource.DataSource  { return &domainDataSource{} }
func NewRouteDataSource() datasource.DataSource   { return &routeDataSource{} }
func NewWebhookDataSource() datasource.DataSource { return &webhookDataSource{} }

func configureDataSource(data any, target **clientData, resp *datasource.ConfigureResponse) {
	requireClient(data, target, &resp.Diagnostics)
}

func (d *teamDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team"
}
func (d *teamDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	configureDataSource(req.ProviderData, &d.client, resp)
}
func (d *teamDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{Description: "Read the current Lettermint team.", Attributes: map[string]schema.Attribute{
		"id": schema.StringAttribute{Computed: true}, "name": schema.StringAttribute{Computed: true},
		"type": schema.StringAttribute{Computed: true}, "plan": schema.StringAttribute{Computed: true},
		"included_volume": schema.Int64Attribute{Computed: true}, "tier": schema.Int64Attribute{Computed: true, DeprecationMessage: "Use included_volume."},
		"verified_at": schema.StringAttribute{Computed: true}, "created_at": schema.StringAttribute{Computed: true},
	}}
}
func (d *teamDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	result, err := d.client.Team.Retrieve(ctx, nil)
	if err != nil {
		appendClientDiagnostic(&resp.Diagnostics, "Cannot read team", err)
		return
	}
	team := *result
	state := teamDataSourceModel{
		ID: stringValue(team.ID), Name: stringValue(team.Name), Type: stringValue(string(team.Type)), Plan: stringValue(string(team.Plan)),
		IncludedVolume: types.Int64Value(int64(team.IncludedVolume)), Tier: types.Int64Value(int64(team.Tier)), VerifiedAt: nullableString(team.VerifiedAt),
		CreatedAt: stringValue(team.CreatedAt),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (d *projectDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}
func (d *projectDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	configureDataSource(req.ProviderData, &d.client, resp)
}
func (d *projectDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{Description: "Read one Lettermint project.", Attributes: map[string]schema.Attribute{
		"id": schema.StringAttribute{Required: true}, "name": schema.StringAttribute{Computed: true}, "smtp_enabled": schema.BoolAttribute{Computed: true},
		"redact_email_content": schema.BoolAttribute{Computed: true}, "default_route_id": schema.StringAttribute{Computed: true},
		"token_generated_at": schema.StringAttribute{Computed: true}, "token_last_used_at": schema.StringAttribute{Computed: true}, "token_last_used_ip": schema.StringAttribute{Computed: true},
		"created_at": schema.StringAttribute{Computed: true}, "updated_at": schema.StringAttribute{Computed: true},
	}}
}
func (d *projectDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config projectDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := d.client.Projects.Retrieve(ctx, config.ID.ValueString(), nil)
	if err != nil {
		appendClientDiagnostic(&resp.Diagnostics, "Cannot read project", err)
		return
	}
	project := *result
	state := projectDataSourceModel{ID: stringValue(project.ID), Name: stringValue(project.Name), SMTPEnabled: types.BoolValue(project.SMTPEnabled), RedactEmailContent: types.BoolValue(project.RedactEmailContent), DefaultRouteID: nullableString(project.DefaultRouteID), TokenGeneratedAt: nullableString(project.TokenGeneratedAt), TokenLastUsedAt: nullableString(project.TokenLastUsedAt), TokenLastUsedIP: nullableString(project.TokenLastUsedIP), CreatedAt: stringValue(project.CreatedAt), UpdatedAt: stringValue(project.UpdatedAt)}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (d *domainDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain"
}
func (d *domainDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	configureDataSource(req.ProviderData, &d.client, resp)
}
func (d *domainDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{Description: "Read one Lettermint domain. The provider does not request DNS record includes.", Attributes: map[string]schema.Attribute{
		"id": schema.StringAttribute{Required: true}, "domain": schema.StringAttribute{Computed: true}, "dkim_mode": schema.StringAttribute{Computed: true},
		"rotation_ready": schema.BoolAttribute{Computed: true}, "status_changed_at": schema.StringAttribute{Computed: true}, "created_at": schema.StringAttribute{Computed: true},
	}}
}
func (d *domainDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config domainDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := d.client.Domains.Retrieve(ctx, config.ID.ValueString(), nil)
	if err != nil {
		appendClientDiagnostic(&resp.Diagnostics, "Cannot read domain", err)
		return
	}
	domain := *result
	state := domainDataSourceModel{ID: stringValue(domain.ID), Domain: stringValue(domain.Domain), DKIMMode: stringValue(string(domain.DkimMode)), RotationReady: types.BoolValue(domain.RotationReady), StatusChangedAt: nullableString(domain.StatusChangedAt), CreatedAt: stringValue(domain.CreatedAt)}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (d *routeDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_route"
}
func (d *routeDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	configureDataSource(req.ProviderData, &d.client, resp)
}
func (d *routeDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attributes := routeDataSourceAttributes()
	resp.Schema = schema.Schema{Description: "Read one Lettermint route.", Attributes: attributes}
}
func (d *routeDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config routeResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := d.client.Routes.Retrieve(ctx, config.ID.ValueString(), nil)
	if err != nil {
		appendClientDiagnostic(&resp.Diagnostics, "Cannot read route", err)
		return
	}
	state := routeModelFromAPI(*result)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func routeDataSourceAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{Required: true}, "project_id": schema.StringAttribute{Computed: true}, "slug": schema.StringAttribute{Computed: true},
		"name": schema.StringAttribute{Computed: true}, "route_type": schema.StringAttribute{Computed: true}, "is_default": schema.BoolAttribute{Computed: true},
		"inbound_address": schema.StringAttribute{Computed: true}, "inbound_domain": schema.StringAttribute{Computed: true}, "inbound_domain_verified_at": schema.StringAttribute{Computed: true},
		"inbound_spam_threshold": schema.Float64Attribute{Computed: true}, "attachment_delivery": schema.StringAttribute{Computed: true},
		"track_opens": schema.BoolAttribute{Computed: true}, "track_clicks": schema.BoolAttribute{Computed: true}, "generate_plaintext_fallback": schema.BoolAttribute{Computed: true},
		"suppress_auto_responders": schema.BoolAttribute{Computed: true}, "suppress_disposable_recipients": schema.BoolAttribute{Computed: true}, "tls": schema.StringAttribute{Computed: true},
		"disable_hosted_unsubscribe": schema.BoolAttribute{Computed: true}, "redact_email_content": schema.BoolAttribute{Computed: true},
		"created_at": schema.StringAttribute{Computed: true}, "updated_at": schema.StringAttribute{Computed: true},
	}
}

func (d *webhookDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webhook"
}
func (d *webhookDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	configureDataSource(req.ProviderData, &d.client, resp)
}
func (d *webhookDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{Description: "Read one route-scoped Lettermint webhook.", Attributes: map[string]schema.Attribute{
		"id": schema.StringAttribute{Required: true}, "route_id": schema.StringAttribute{Computed: true}, "name": schema.StringAttribute{Computed: true}, "url": schema.StringAttribute{Computed: true},
		"events": schema.ListAttribute{Computed: true, ElementType: types.StringType}, "enabled": schema.BoolAttribute{Computed: true}, "include_machine_events": schema.BoolAttribute{Computed: true},
		"has_basic_auth": schema.BoolAttribute{Computed: true, Description: "Whether the webhook has Basic Auth credentials. The API does not return the credentials."},
		"last_called_at": schema.StringAttribute{Computed: true}, "created_at": schema.StringAttribute{Computed: true}, "updated_at": schema.StringAttribute{Computed: true},
	}}
}
func (d *webhookDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config webhookDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := d.client.Webhooks.Retrieve(ctx, config.ID.ValueString())
	if err != nil {
		appendClientDiagnostic(&resp.Diagnostics, "Cannot read webhook", err)
		return
	}
	webhook := *result
	if webhook.RouteID == nil || *webhook.RouteID == "" {
		resp.Diagnostics.AddError("Unsupported webhook scope", "This data source reads route-scoped webhooks only.")
		return
	}
	state := webhookDataSourceModelFromAPI(webhook)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
