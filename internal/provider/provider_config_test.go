package provider

import (
	"context"
	"testing"

	frameworkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lettermint/lettermint-go/v3"
)

func configuredProviderToken(t *testing.T, token types.String) (string, bool) {
	t.Helper()
	underTest := &lettermintProvider{version: "test"}
	captured := ""
	underTest.clientFactory = func(token string) (*lettermint.Client, error) {
		captured = token
		return lettermint.New(lettermint.WithTeamToken(token))
	}
	var schemaResponse frameworkprovider.SchemaResponse
	underTest.Schema(context.Background(), frameworkprovider.SchemaRequest{}, &schemaResponse)
	state := tfsdk.State{Schema: schemaResponse.Schema}
	if diagnostics := state.Set(context.Background(), &providerModel{TeamToken: token}); diagnostics.HasError() {
		t.Fatalf("config diagnostics = %v", diagnostics)
	}
	request := frameworkprovider.ConfigureRequest{Config: tfsdk.Config{Schema: schemaResponse.Schema, Raw: state.Raw}}
	var response frameworkprovider.ConfigureResponse
	underTest.Configure(context.Background(), request, &response)
	return captured, response.Diagnostics.HasError()
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
