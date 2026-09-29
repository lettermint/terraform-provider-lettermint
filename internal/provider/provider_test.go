package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

func TestProtocol6SchemaIsValid(t *testing.T) {
	t.Parallel()

	server := providerserver.NewProtocol6(New("test")())()
	response, err := server.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatalf("GetProviderSchema() error = %v", err)
	}
	for _, diagnostic := range response.Diagnostics {
		if diagnostic.Severity == tfprotov6.DiagnosticSeverityError {
			t.Errorf("schema error: %s: %s", diagnostic.Summary, diagnostic.Detail)
		}
	}
}

func TestProviderSurfaceIsCurrentSDKSubset(t *testing.T) {
	t.Parallel()

	instance := New("test")().(*lettermintProvider)
	if got := len(instance.Resources(context.Background())); got != 4 {
		t.Fatalf("resource count = %d, want 4", got)
	}
	if got := len(instance.DataSources(context.Background())); got != 9 {
		t.Fatalf("data source count = %d, want 9", got)
	}
	if got := len(instance.Actions(context.Background())); got != 3 {
		t.Fatalf("action count = %d, want 3", got)
	}

	var projectSchema resource.SchemaResponse
	(&projectResource{}).Schema(context.Background(), resource.SchemaRequest{}, &projectSchema)
	for _, excluded := range []string{"delivery_mode", "routes", "domains", "last_28_days"} {
		if _, ok := projectSchema.Schema.Attributes[excluded]; ok {
			t.Errorf("project field %q must not be exposed", excluded)
		}
	}

	var webhookSchema resource.SchemaResponse
	(&webhookResource{}).Schema(context.Background(), resource.SchemaRequest{}, &webhookSchema)
	for _, excluded := range []string{"scope", "project_ids", "route_ids", "delivery_mode_filter"} {
		if _, ok := webhookSchema.Schema.Attributes[excluded]; ok {
			t.Errorf("webhook field %q must not be exposed", excluded)
		}
	}

	var domainSchema resource.SchemaResponse
	(&domainResource{}).Schema(context.Background(), resource.SchemaRequest{}, &domainSchema)
	dnsRecords, ok := domainSchema.Schema.Attributes["dns_records"].(resourceschema.ListNestedAttribute)
	if !ok {
		t.Fatal("dns_records is not a nested list")
	}
	if got := len(dnsRecords.NestedObject.Attributes); got != 11 {
		t.Fatalf("dns_records field count = %d, want 11", got)
	}

	var webhookDataSourceSchema datasource.SchemaResponse
	(&webhookDataSource{}).Schema(context.Background(), datasource.SchemaRequest{}, &webhookDataSourceSchema)
	if _, ok := webhookDataSourceSchema.Schema.Attributes["secret"]; ok {
		t.Error("webhook data source must not expose the create-only secret")
	}

	var teamDataSourceSchema datasource.SchemaResponse
	(&teamDataSource{}).Schema(context.Background(), datasource.SchemaRequest{}, &teamDataSourceSchema)
	for _, sdkBlocked := range []string{"domains_count", "projects_count", "members_count"} {
		if _, ok := teamDataSourceSchema.Schema.Attributes[sdkBlocked]; ok {
			t.Errorf("team field %q must not be exposed because the SDK loses optional presence", sdkBlocked)
		}
	}
}

func TestProviderSchemaMarksTeamTokenSensitive(t *testing.T) {
	t.Parallel()

	instance := New("test")().(*lettermintProvider)
	var response provider.SchemaResponse
	instance.Schema(context.Background(), provider.SchemaRequest{}, &response)

	attribute, ok := response.Schema.Attributes["team_token"]
	if !ok {
		t.Fatal("team_token is absent")
	}
	if !attribute.IsSensitive() {
		t.Fatal("team_token is not sensitive")
	}
}
