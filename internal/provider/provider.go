package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	providerschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lettermint/lettermint-go/v2"
)

const typeName = "lettermint"

var (
	_ provider.Provider            = (*lettermintProvider)(nil)
	_ provider.ProviderWithActions = (*lettermintProvider)(nil)
)

type lettermintProvider struct {
	version       string
	clientFactory func(string) (*lettermint.APIClient, error)
}

type providerModel struct {
	TeamToken types.String `tfsdk:"team_token"`
}

type clientData struct {
	API *lettermint.APIClient
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &lettermintProvider{
			version: version,
			clientFactory: func(token string) (*lettermint.APIClient, error) {
				return lettermint.NewAPI(token)
			},
		}
	}
}

func (p *lettermintProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = typeName
	resp.Version = p.version
}

func (p *lettermintProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = providerschema.Schema{
		Description: "Manage Lettermint projects, domains, routes, and route webhooks.",
		Attributes: map[string]providerschema.Attribute{
			"team_token": providerschema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Lettermint team token. You can also set LETTERMINT_TEAM_TOKEN.",
			},
		},
	}
}

func (p *lettermintProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	token := os.Getenv("LETTERMINT_TEAM_TOKEN")
	if !config.TeamToken.IsNull() && !config.TeamToken.IsUnknown() {
		token = config.TeamToken.ValueString()
	}
	if token == "" {
		resp.Diagnostics.AddError(
			"Missing Lettermint team token",
			"Set team_token in the provider configuration or set LETTERMINT_TEAM_TOKEN.",
		)
		return
	}

	clientFactory := p.clientFactory
	if clientFactory == nil {
		clientFactory = func(token string) (*lettermint.APIClient, error) {
			return lettermint.NewAPI(token)
		}
	}
	api, err := clientFactory(token)
	if err != nil {
		resp.Diagnostics.AddError("Cannot create Lettermint client", err.Error())
		return
	}

	data := &clientData{API: api}
	resp.ResourceData = data
	resp.DataSourceData = data
	resp.ActionData = data
}

func (p *lettermintProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewProjectResource,
		NewDomainResource,
		NewRouteResource,
		NewWebhookResource,
	}
}

func (p *lettermintProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewTeamDataSource,
		NewProjectDataSource,
		NewProjectsDataSource,
		NewDomainDataSource,
		NewDomainsDataSource,
		NewRouteDataSource,
		NewRoutesDataSource,
		NewWebhookDataSource,
		NewWebhooksDataSource,
	}
}

func (p *lettermintProvider) Actions(_ context.Context) []func() action.Action {
	return []func() action.Action{
		NewVerifyDomainDNSAction,
		NewVerifyDomainDNSRecordAction,
		NewVerifyRouteInboundDomainAction,
	}
}

func appendClientDiagnostic(diags *diag.Diagnostics, summary string, err error) {
	if err != nil {
		diags.AddError(summary, err.Error())
	}
}
