package provider

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type projectsDataSource struct{ client *clientData }
type domainsDataSource struct{ client *clientData }
type routesDataSource struct{ client *clientData }
type webhooksDataSource struct{ client *clientData }

type projectsDataSourceModel struct {
	Search   types.String       `tfsdk:"search"`
	Sort     types.List         `tfsdk:"sort"`
	PageSize types.Int64        `tfsdk:"page_size"`
	Projects []projectListModel `tfsdk:"projects"`
}
type projectListModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	SMTPEnabled  types.Bool   `tfsdk:"smtp_enabled"`
	RoutesCount  types.Int64  `tfsdk:"routes_count"`
	DomainsCount types.Int64  `tfsdk:"domains_count"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
}

type domainsDataSourceModel struct {
	ProjectID types.String      `tfsdk:"project_id"`
	Status    types.String      `tfsdk:"status"`
	Domain    types.String      `tfsdk:"domain"`
	Sort      types.List        `tfsdk:"sort"`
	PageSize  types.Int64       `tfsdk:"page_size"`
	Domains   []domainListModel `tfsdk:"domains"`
}
type domainListModel struct {
	ID              types.String `tfsdk:"id"`
	Domain          types.String `tfsdk:"domain"`
	Status          types.String `tfsdk:"status"`
	DKIMMode        types.String `tfsdk:"dkim_mode"`
	StatusChangedAt types.String `tfsdk:"status_changed_at"`
	CreatedAt       types.String `tfsdk:"created_at"`
}

type routesDataSourceModel struct {
	ProjectID types.String     `tfsdk:"project_id"`
	RouteType types.String     `tfsdk:"route_type"`
	IsDefault types.Bool       `tfsdk:"is_default"`
	Search    types.String     `tfsdk:"search"`
	Sort      types.List       `tfsdk:"sort"`
	PageSize  types.Int64      `tfsdk:"page_size"`
	Routes    []routeListModel `tfsdk:"routes"`
}
type routeListModel struct {
	ID                        types.String `tfsdk:"id"`
	Slug                      types.String `tfsdk:"slug"`
	Name                      types.String `tfsdk:"name"`
	RouteType                 types.String `tfsdk:"route_type"`
	IsDefault                 types.Bool   `tfsdk:"is_default"`
	WebhooksCount             types.Int64  `tfsdk:"webhooks_count"`
	SuppressedRecipientsCount types.Int64  `tfsdk:"suppressed_recipients_count"`
	CreatedAt                 types.String `tfsdk:"created_at"`
	UpdatedAt                 types.String `tfsdk:"updated_at"`
}

type webhooksDataSourceModel struct {
	RouteID  types.String       `tfsdk:"route_id"`
	Enabled  types.Bool         `tfsdk:"enabled"`
	Event    types.String       `tfsdk:"event"`
	Search   types.String       `tfsdk:"search"`
	Sort     types.List         `tfsdk:"sort"`
	Webhooks []webhookListModel `tfsdk:"webhooks"`
}
type webhookListModel struct {
	ID           types.String `tfsdk:"id"`
	RouteID      types.String `tfsdk:"route_id"`
	Name         types.String `tfsdk:"name"`
	URL          types.String `tfsdk:"url"`
	Events       types.List   `tfsdk:"events"`
	Enabled      types.Bool   `tfsdk:"enabled"`
	HasBasicAuth types.Bool   `tfsdk:"has_basic_auth"`
	LastCalledAt types.String `tfsdk:"last_called_at"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
}

func NewProjectsDataSource() datasource.DataSource { return &projectsDataSource{} }
func NewDomainsDataSource() datasource.DataSource  { return &domainsDataSource{} }
func NewRoutesDataSource() datasource.DataSource   { return &routesDataSource{} }
func NewWebhooksDataSource() datasource.DataSource { return &webhooksDataSource{} }

func pageSizeAttribute() schema.Int64Attribute {
	return schema.Int64Attribute{Optional: true, Description: "Results requested per API page. The API default is 30."}
}
func sortAttribute(values ...string) schema.ListAttribute {
	return schema.ListAttribute{
		Optional:    true,
		ElementType: types.StringType,
		Description: "Sort values in API order. The API default is -created_at.",
		Validators:  []validator.List{listvalidator.ValueStringsAre(stringvalidator.OneOf(values...))},
	}
}
func addPageSize(query map[string]string, size types.Int64) {
	if !size.IsNull() && !size.IsUnknown() {
		query["page[size]"] = strconv.FormatInt(size.ValueInt64(), 10)
	}
}
func addStringQuery(query map[string]string, key string, value types.String) {
	if !value.IsNull() && !value.IsUnknown() {
		query[key] = value.ValueString()
	}
}

func (d *projectsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_projects"
}
func (d *projectsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	configureDataSource(req.ProviderData, &d.client, resp)
}
func (d *projectsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{Description: "List Lettermint projects.", Attributes: map[string]schema.Attribute{
		"search": schema.StringAttribute{Optional: true}, "sort": sortAttribute("name", "-name", "created_at", "-created_at"), "page_size": pageSizeAttribute(),
		"projects": schema.ListNestedAttribute{Computed: true, NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true}, "name": schema.StringAttribute{Computed: true}, "smtp_enabled": schema.BoolAttribute{Computed: true},
			"routes_count": schema.Int64Attribute{Computed: true}, "domains_count": schema.Int64Attribute{Computed: true}, "created_at": schema.StringAttribute{Computed: true}, "updated_at": schema.StringAttribute{Computed: true},
		}}},
	}}
}
func (d *projectsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config projectsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	query := map[string]string{}
	config.Projects = []projectListModel{}
	addStringQuery(query, "filter[search]", config.Search)
	addPageSize(query, config.PageSize)
	if sort, configured := queryList(ctx, config.Sort, &resp.Diagnostics); configured {
		query["sort"] = sort
	}
	for {
		result, err := d.client.API.Projects.List(ctx, query)
		if err != nil {
			appendClientDiagnostic(&resp.Diagnostics, "Cannot list projects", err)
			return
		}
		for _, item := range result.Data {
			config.Projects = append(config.Projects, projectListModel{ID: stringValue(item.ID), Name: stringValue(item.Name), SMTPEnabled: types.BoolValue(item.SMTPEnabled), RoutesCount: types.Int64Value(int64(item.RoutesCount)), DomainsCount: types.Int64Value(int64(item.DomainsCount)), CreatedAt: stringValue(item.CreatedAt), UpdatedAt: stringValue(item.UpdatedAt)})
		}
		if result.NextCursor == nil {
			break
		}
		query["page[cursor]"] = *result.NextCursor
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func (d *domainsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domains"
}
func (d *domainsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	configureDataSource(req.ProviderData, &d.client, resp)
}
func (d *domainsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{Description: "List Lettermint domains.", Attributes: map[string]schema.Attribute{
		"project_id": schema.StringAttribute{Optional: true},
		"status":     schema.StringAttribute{Optional: true, Validators: enum("verified", "partially_verified", "pending_verification", "failed_verification")},
		"domain":     schema.StringAttribute{Optional: true},
		"sort":       sortAttribute("domain", "-domain", "created_at", "-created_at", "status_changed_at", "-status_changed_at"),
		"page_size":  pageSizeAttribute(),
		"domains": schema.ListNestedAttribute{Computed: true, NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true}, "domain": schema.StringAttribute{Computed: true}, "status": schema.StringAttribute{Computed: true}, "dkim_mode": schema.StringAttribute{Computed: true}, "status_changed_at": schema.StringAttribute{Computed: true}, "created_at": schema.StringAttribute{Computed: true},
		}}},
	}}
}
func (d *domainsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config domainsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	query := map[string]string{}
	config.Domains = []domainListModel{}
	addStringQuery(query, "filter[project]", config.ProjectID)
	addStringQuery(query, "filter[status]", config.Status)
	addStringQuery(query, "filter[domain]", config.Domain)
	addPageSize(query, config.PageSize)
	if sort, configured := queryList(ctx, config.Sort, &resp.Diagnostics); configured {
		query["sort"] = sort
	}
	for {
		result, err := d.client.API.Domains.List(ctx, query)
		if err != nil {
			appendClientDiagnostic(&resp.Diagnostics, "Cannot list domains", err)
			return
		}
		for _, item := range result.Data {
			config.Domains = append(config.Domains, domainListModel{ID: stringValue(item.ID), Domain: stringValue(item.Domain), Status: stringValue(string(item.Status)), DKIMMode: stringValue(string(item.DkimMode)), StatusChangedAt: nullableString(item.StatusChangedAt), CreatedAt: stringValue(item.CreatedAt)})
		}
		if result.NextCursor == nil {
			break
		}
		query["page[cursor]"] = *result.NextCursor
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func (d *routesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_routes"
}
func (d *routesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	configureDataSource(req.ProviderData, &d.client, resp)
}
func (d *routesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{Description: "List routes for one Lettermint project.", Attributes: map[string]schema.Attribute{
		"project_id": schema.StringAttribute{Required: true},
		"route_type": schema.StringAttribute{Optional: true, Validators: enum("transactional", "broadcast", "inbound")},
		"is_default": schema.BoolAttribute{Optional: true},
		"search":     schema.StringAttribute{Optional: true},
		"sort":       sortAttribute("name", "-name", "slug", "-slug", "created_at", "-created_at"),
		"page_size":  pageSizeAttribute(),
		"routes": schema.ListNestedAttribute{Computed: true, NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true}, "slug": schema.StringAttribute{Computed: true}, "name": schema.StringAttribute{Computed: true}, "route_type": schema.StringAttribute{Computed: true}, "is_default": schema.BoolAttribute{Computed: true},
			"webhooks_count": schema.Int64Attribute{Computed: true}, "suppressed_recipients_count": schema.Int64Attribute{Computed: true}, "created_at": schema.StringAttribute{Computed: true}, "updated_at": schema.StringAttribute{Computed: true},
		}}},
	}}
}
func (d *routesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config routesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	query := map[string]string{}
	config.Routes = []routeListModel{}
	addStringQuery(query, "filter[route_type]", config.RouteType)
	addStringQuery(query, "filter[search]", config.Search)
	addPageSize(query, config.PageSize)
	if !config.IsDefault.IsNull() && !config.IsDefault.IsUnknown() {
		query["filter[is_default]"] = queryBool(config.IsDefault)
	}
	if sort, configured := queryList(ctx, config.Sort, &resp.Diagnostics); configured {
		query["sort"] = sort
	}
	for {
		result, err := d.client.API.Projects.Routes(ctx, config.ProjectID.ValueString(), query)
		if err != nil {
			appendClientDiagnostic(&resp.Diagnostics, "Cannot list routes", err)
			return
		}
		for _, item := range result.Data {
			config.Routes = append(config.Routes, routeListModel{ID: stringValue(item.ID), Slug: stringValue(item.Slug), Name: stringValue(item.Name), RouteType: stringValue(string(item.RouteType)), IsDefault: types.BoolValue(item.IsDefault), WebhooksCount: types.Int64Value(int64(item.WebhooksCount)), SuppressedRecipientsCount: types.Int64Value(int64(item.SuppressedRecipientsCount)), CreatedAt: stringValue(item.CreatedAt), UpdatedAt: stringValue(item.UpdatedAt)})
		}
		if result.NextCursor == nil {
			break
		}
		query["page[cursor]"] = *result.NextCursor
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func (d *webhooksDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webhooks"
}
func (d *webhooksDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	configureDataSource(req.ProviderData, &d.client, resp)
}
func (d *webhooksDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{Description: "List route-scoped Lettermint webhooks.", Attributes: map[string]schema.Attribute{
		"route_id": schema.StringAttribute{Required: true, Validators: uuidValidators()},
		"enabled":  schema.BoolAttribute{Optional: true},
		"event": schema.StringAttribute{Optional: true, Validators: enum(
			"message.created", "message.sent", "message.delivered", "message.auto_replied",
			"message.hard_bounced", "message.soft_bounced", "message.spam_complaint", "message.failed",
			"message.suppressed", "message.unsubscribed", "message.opened", "message.clicked",
			"message.inbound", "message.policy_rejected", "message.scheduled", "message.rescheduled",
			"message.canceled", "message.released", "suppression.added", "suppression.removed", "webhook.test",
		)},
		"search": schema.StringAttribute{Optional: true},
		"sort":   sortAttribute("name", "-name", "url", "-url", "created_at", "-created_at"),
		"webhooks": schema.ListNestedAttribute{Computed: true, NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true}, "route_id": schema.StringAttribute{Computed: true}, "name": schema.StringAttribute{Computed: true}, "url": schema.StringAttribute{Computed: true}, "events": schema.ListAttribute{Computed: true, ElementType: types.StringType},
			"enabled": schema.BoolAttribute{Computed: true}, "has_basic_auth": schema.BoolAttribute{Computed: true, Description: "Whether the webhook has Basic Auth credentials."}, "last_called_at": schema.StringAttribute{Computed: true}, "created_at": schema.StringAttribute{Computed: true}, "updated_at": schema.StringAttribute{Computed: true},
		}}},
	}}
}
func (d *webhooksDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config webhooksDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	query := map[string]string{"filter[route_id]": config.RouteID.ValueString()}
	config.Webhooks = []webhookListModel{}
	addStringQuery(query, "filter[event]", config.Event)
	addStringQuery(query, "filter[search]", config.Search)
	if !config.Enabled.IsNull() && !config.Enabled.IsUnknown() {
		query["filter[enabled]"] = queryBool(config.Enabled)
	}
	if sort, configured := queryList(ctx, config.Sort, &resp.Diagnostics); configured {
		query["sort"] = sort
	}
	result, err := d.client.API.Webhooks.List(ctx, query)
	if err != nil {
		appendClientDiagnostic(&resp.Diagnostics, "Cannot list webhooks", err)
		return
	}
	for _, item := range result.Data {
		if item.RouteID == nil || *item.RouteID != config.RouteID.ValueString() {
			continue
		}
		eventValues := make([]attr.Value, 0, len(item.Events))
		for _, event := range item.Events {
			eventValues = append(eventValues, types.StringValue(string(event)))
		}
		config.Webhooks = append(config.Webhooks, webhookListModel{ID: stringValue(item.ID), RouteID: nullableString(item.RouteID), Name: stringValue(item.Name), URL: stringValue(item.URL), Events: types.ListValueMust(types.StringType, eventValues), Enabled: types.BoolValue(item.Enabled), HasBasicAuth: types.BoolValue(item.HasBasicAuth), LastCalledAt: nullableString(item.LastCalledAt), CreatedAt: stringValue(item.CreatedAt), UpdatedAt: stringValue(item.UpdatedAt)})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
