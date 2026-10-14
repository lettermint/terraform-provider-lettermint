package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	providerschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lettermint/lettermint-go/v3"
)

const typeName = "lettermint"

var (
	_ provider.Provider            = (*lettermintProvider)(nil)
	_ provider.ProviderWithActions = (*lettermintProvider)(nil)
)

type lettermintProvider struct {
	version       string
	clientFactory func(clientConfig) (*lettermint.Client, error)
}

type providerModel struct {
	TeamToken  types.String `tfsdk:"team_token"`
	MaxRetries types.Int64  `tfsdk:"max_retries"`
}

// clientConfig holds the provider settings that the Lettermint client uses.
type clientConfig struct {
	Token      string
	MaxRetries int
}

type clientData struct {
	*lettermint.Client
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &lettermintProvider{
			version:       version,
			clientFactory: newClient,
		}
	}
}

// newClient creates a Lettermint client that uses the team token for the Team
// API and retries rate-limited requests.
func newClient(config clientConfig) (*lettermint.Client, error) {
	transport := newRateLimitTransport(http.DefaultTransport, config.MaxRetries)
	return lettermint.New(
		lettermint.WithTeamToken(config.Token),
		lettermint.WithTimeout(transport.callTimeout()),
		lettermint.WithHTTPClient(&http.Client{Transport: transport}),
	)
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
			"max_retries": providerschema.Int64Attribute{
				Optional:    true,
				Description: "How often to retry a request that the Lettermint API rejects with HTTP 429 Too Many Requests. The provider waits for the Retry-After duration before each retry. Defaults to 8, at most 100. Set 0 to turn retries off. You can also set LETTERMINT_MAX_RETRIES.",
				Validators:  []validator.Int64{int64validator.Between(0, maxRetriesLimit)},
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

	maxRetries := defaultMaxRetries
	if !config.MaxRetries.IsNull() && !config.MaxRetries.IsUnknown() {
		maxRetries = int(config.MaxRetries.ValueInt64())
	} else if value := os.Getenv("LETTERMINT_MAX_RETRIES"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 || parsed > maxRetriesLimit {
			resp.Diagnostics.AddAttributeError(
				path.Root("max_retries"),
				"Invalid LETTERMINT_MAX_RETRIES",
				fmt.Sprintf("LETTERMINT_MAX_RETRIES must be a whole number from 0 to %d.", maxRetriesLimit),
			)
			return
		}
		maxRetries = parsed
	}

	clientFactory := p.clientFactory
	if clientFactory == nil {
		clientFactory = newClient
	}
	client, err := clientFactory(clientConfig{Token: token, MaxRetries: maxRetries})
	if err != nil {
		resp.Diagnostics.AddError("Cannot create Lettermint client", err.Error())
		return
	}

	data := &clientData{Client: client}
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
	if err == nil {
		return
	}
	var rateLimit *lettermint.RateLimitError
	if errors.As(err, &rateLimit) {
		diags.AddError(summary, err.Error()+"\n\nThe Lettermint API rate limit was reached. "+
			"Wait a minute and run Terraform again, lower -parallelism, or raise max_retries.")
		return
	}
	diags.AddError(summary, err.Error())
}
