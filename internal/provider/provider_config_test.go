package provider

import (
	"context"
	"testing"

	frameworkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lettermint/lettermint-go/v3"
)

func configuredProvider(t *testing.T, model providerModel) (clientConfig, bool) {
	t.Helper()
	underTest := &lettermintProvider{version: "test"}
	var captured clientConfig
	underTest.clientFactory = func(config clientConfig) (*lettermint.Client, error) {
		captured = config
		return lettermint.New(lettermint.WithTeamToken(config.Token))
	}
	var schemaResponse frameworkprovider.SchemaResponse
	underTest.Schema(context.Background(), frameworkprovider.SchemaRequest{}, &schemaResponse)
	state := tfsdk.State{Schema: schemaResponse.Schema}
	if diagnostics := state.Set(context.Background(), &model); diagnostics.HasError() {
		t.Fatalf("config diagnostics = %v", diagnostics)
	}
	request := frameworkprovider.ConfigureRequest{Config: tfsdk.Config{Schema: schemaResponse.Schema, Raw: state.Raw}}
	var response frameworkprovider.ConfigureResponse
	underTest.Configure(context.Background(), request, &response)
	return captured, response.Diagnostics.HasError()
}

func configuredProviderToken(t *testing.T, token types.String) (string, bool) {
	t.Helper()
	config, hasError := configuredProvider(t, providerModel{TeamToken: token, MaxRetries: types.Int64Null()})
	return config.Token, hasError
}

func TestProviderTeamTokenConfiguration(t *testing.T) {
	t.Setenv("LETTERMINT_TEAM_TOKEN", "environment-token")

	if token, hasError := configuredProviderToken(t, types.StringNull()); hasError || token != "environment-token" {
		t.Fatalf("environment token = %q, diagnostics error = %t", token, hasError)
	}
	if token, hasError := configuredProviderToken(t, types.StringValue("configuration-token")); hasError || token != "configuration-token" {
		t.Fatalf("configuration token = %q, diagnostics error = %t", token, hasError)
	}
}

func TestProviderRequiresTeamToken(t *testing.T) {
	t.Setenv("LETTERMINT_TEAM_TOKEN", "")

	if _, hasError := configuredProviderToken(t, types.StringNull()); !hasError {
		t.Fatal("missing token did not return an error")
	}
}

func TestProviderMaxRetriesConfiguration(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		configured  types.Int64
		want        int
		wantError   bool
	}{
		{name: "default", configured: types.Int64Null(), want: defaultMaxRetries},
		{name: "environment", environment: "3", configured: types.Int64Null(), want: 3},
		{name: "configuration overrides environment", environment: "3", configured: types.Int64Value(0), want: 0},
		{name: "invalid environment", environment: "many", configured: types.Int64Null(), wantError: true},
		{name: "negative environment", environment: "-1", configured: types.Int64Null(), wantError: true},
		{name: "environment above the limit", environment: "101", configured: types.Int64Null(), wantError: true},
		{name: "configuration ignores invalid environment", environment: "many", configured: types.Int64Value(2), want: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("LETTERMINT_MAX_RETRIES", test.environment)

			config, hasError := configuredProvider(t, providerModel{TeamToken: types.StringValue("token"), MaxRetries: test.configured})
			if hasError != test.wantError {
				t.Fatalf("diagnostics error = %t, want %t", hasError, test.wantError)
			}
			if !test.wantError && config.MaxRetries != test.want {
				t.Fatalf("max retries = %d, want %d", config.MaxRetries, test.want)
			}
		})
	}
}
