package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	testresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/lettermint/lettermint-go/v2"
)

func testBasicAuth(username, password types.String) types.Object {
	return types.ObjectValueMust(webhookBasicAuthTypes(), map[string]attr.Value{"username": username, "password": password})
}

func TestWebhookBasicAuthRequestStates(t *testing.T) {
	for _, test := range []struct {
		name                              string
		value                             types.Object
		version, previous                 types.Int64
		update, omitted, cleared, invalid bool
	}{
		{name: "create omitted", value: types.ObjectNull(webhookBasicAuthTypes()), version: types.Int64Null(), previous: types.Int64Null(), omitted: true},
		{name: "set empty password", value: testBasicAuth(types.StringValue(" user "), types.StringValue("")), version: types.Int64Value(1), previous: types.Int64Null()},
		{name: "update retained", value: types.ObjectNull(webhookBasicAuthTypes()), version: types.Int64Value(1), previous: types.Int64Value(1), update: true, omitted: true},
		{name: "update cleared", value: types.ObjectNull(webhookBasicAuthTypes()), version: types.Int64Value(2), previous: types.Int64Value(1), update: true, cleared: true},
		{name: "version removed clears", value: types.ObjectNull(webhookBasicAuthTypes()), version: types.Int64Null(), previous: types.Int64Value(1), update: true, cleared: true},
		{name: "missing version", value: testBasicAuth(types.StringValue(" user "), types.StringValue("")), version: types.Int64Null(), previous: types.Int64Null(), invalid: true},
		{name: "unknown password", value: testBasicAuth(types.StringValue(" user "), types.StringUnknown()), version: types.Int64Value(1), previous: types.Int64Null(), invalid: true},
		{name: "null username", value: testBasicAuth(types.StringNull(), types.StringValue("")), version: types.Int64Value(1), previous: types.Int64Null(), invalid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var diagnostics diag.Diagnostics
			got := webhookBasicAuthRequest(test.value, test.version, test.previous, test.update, &diagnostics)
			if diagnostics.HasError() != test.invalid {
				t.Fatal("unexpected credential diagnostics")
			}
			if test.invalid {
				return
			}
			if test.omitted {
				if got != nil {
					t.Fatal("credentials were not omitted")
				}
				return
			}
			if got == nil {
				t.Fatal("missing credential request")
			}
			if test.cleared {
				if *got != nil {
					t.Fatal("credentials were not cleared")
				}
				return
			}
			if *got == nil || (**got).Username != " user " || (**got).Password != "" {
				t.Fatal("credential strings changed")
			}
		})
	}
}

func TestWebhookCredentialSchemaIsWriteOnly(t *testing.T) {
	s := webhookResourceSchema()
	a := s.Attributes["basic_auth"].(schema.SingleNestedAttribute)
	if !a.WriteOnly || !a.Sensitive || !a.Optional {
		t.Fatal("credential object must be optional, sensitive and write-only")
	}
	for _, name := range []string{"username", "password"} {
		field := a.Attributes[name].(schema.StringAttribute)
		if !field.WriteOnly || !field.Sensitive || !field.Required {
			t.Fatal("credential fields must be required, sensitive and write-only")
		}
	}
	if !s.Attributes["has_basic_auth"].IsComputed() {
		t.Fatal("safe read flag must be computed")
	}
}

type basicAuthTransport func(*http.Request) (*http.Response, error)

func (f basicAuthTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestWebhookCredentialsDoNotEnterState(t *testing.T) {
	ctx := context.Background()
	s := webhookResourceSchema()
	nullAuth := types.ObjectNull(webhookBasicAuthTypes())
	credentials := testBasicAuth(types.StringValue(" user "), types.StringValue(""))
	version := types.Int64Value(1)
	model := webhookResourceModel{
		ID: types.StringValue(testWebhookID), RouteID: types.StringValue(testRouteID),
		Name: types.StringValue("Delivery events"), URL: types.StringValue("https://example.com/webhook"),
		Events:  types.ListValueMust(types.StringType, []attr.Value{types.StringValue("message.delivered")}),
		Enabled: types.BoolValue(true), IncludeMachineEvents: types.BoolValue(false),
		BasicAuth: nullAuth, BasicAuthVersion: version, HasBasicAuth: types.BoolValue(false),
		Secret: types.StringNull(), LastCalledAt: types.StringNull(), CreatedAt: types.StringNull(), UpdatedAt: types.StringNull(),
	}
	for _, mode := range []string{"create", "retain", "rotate", "clear"} {
		t.Run(mode, func(t *testing.T) {
			planModel, configModel, stateModel := model, model, model
			expectedAuth := any(map[string]any{"username": " user ", "password": ""})
			if mode == "create" || mode == "rotate" {
				configModel.BasicAuth = credentials
			}
			if mode == "rotate" || mode == "clear" {
				planModel.BasicAuthVersion = types.Int64Value(2)
				configModel.BasicAuthVersion = planModel.BasicAuthVersion
			}
			if mode == "retain" || mode == "clear" {
				expectedAuth = nil
			}
			planState, configState, priorState := tfsdk.State{Schema: s}, tfsdk.State{Schema: s}, tfsdk.State{Schema: s}
			for target, value := range map[*tfsdk.State]*webhookResourceModel{&planState: &planModel, &configState: &configModel, &priorState: &stateModel} {
				if d := target.Set(ctx, value); d.HasError() {
					t.Fatal(d)
				}
			}
			calls := 0
			api, err := lettermint.NewAPI("test-token", lettermint.WithBaseURL("https://api.example.test/v1"), lettermint.WithHTTPClient(&http.Client{Transport: basicAuthTransport(func(request *http.Request) (*http.Response, error) {
				calls++
				wantMethod, wantPath := http.MethodPut, "/v1/webhooks/"+testWebhookID
				if mode == "create" {
					wantMethod, wantPath = http.MethodPost, "/v1/webhooks"
				}
				if request.Method != wantMethod || request.URL.Path != wantPath || request.Header.Get("Authorization") != "Bearer test-token" || request.Header.Get("Content-Type") != "application/json" {
					t.Fatal("incorrect webhook request boundary")
				}
				body := requestJSON(request)
				got, present := body["basic_auth"]
				if present != (mode != "retain") || !reflect.DeepEqual(got, expectedAuth) {
					t.Fatal("incorrect credential request state")
				}
				data := map[string]any{"id": testWebhookID, "scope": "route", "route_id": testRouteID, "project_ids": []any{}, "route_ids": []any{testRouteID}, "name": "Delivery events", "url": "https://example.com/webhook", "events": []any{"message.delivered"}, "enabled": true, "include_machine_events": false, "delivery_mode_filter": "all", "has_basic_auth": mode != "clear", "last_called_at": nil, "created_at": "2026-10-02T00:00:00Z", "updated_at": "2026-10-02T00:00:00Z"}
				status := http.StatusOK
				if mode == "create" {
					status = http.StatusCreated
					data["secret"] = "whsec_test"
				}
				return apiResponse(request, status, map[string]any{"data": data, "message": "Success"})
			})}))
			if err != nil {
				t.Fatal(err)
			}
			r := &webhookResource{client: &clientData{API: api}}
			var output tfsdk.State
			if mode == "create" {
				resp := resource.CreateResponse{State: tfsdk.State{Schema: s}}
				r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan{Schema: s, Raw: planState.Raw}, Config: tfsdk.Config{Schema: s, Raw: configState.Raw}}, &resp)
				if resp.Diagnostics.HasError() {
					t.Fatal(resp.Diagnostics)
				}
				output = resp.State
			} else {
				resp := resource.UpdateResponse{State: tfsdk.State{Schema: s}}
				r.Update(ctx, resource.UpdateRequest{Plan: tfsdk.Plan{Schema: s, Raw: planState.Raw}, Config: tfsdk.Config{Schema: s, Raw: configState.Raw}, State: priorState}, &resp)
				if resp.Diagnostics.HasError() {
					t.Fatal(resp.Diagnostics)
				}
				output = resp.State
			}
			var stored webhookResourceModel
			if d := output.Get(ctx, &stored); d.HasError() {
				t.Fatal(d)
			}
			if calls != 1 || !stored.BasicAuth.IsNull() || stored.HasBasicAuth.ValueBool() != (mode != "clear") {
				t.Fatal("credentials entered state or safe flag changed")
			}
			if mode == "create" && stored.Secret.ValueString() != "whsec_test" {
				t.Fatal("create signing secret was lost")
			}
			encoded, err := json.Marshal(stored.BasicAuth.Attributes())
			if err != nil || string(encoded) != "{}" {
				t.Fatal("credential state is not empty")
			}
		})
	}
}

func TestWebhookSafeFlagHydration(t *testing.T) {
	for _, flag := range []bool{false, true} {
		value := webhookDataSourceModelFromAPI(lettermint.WebhookData{HasBasicAuth: flag})
		if value.HasBasicAuth.ValueBool() != flag || !value.RouteID.IsNull() {
			t.Fatal("safe flag or nullable route ID changed")
		}
	}
}

func TestAccWebhookBasicAuth(t *testing.T) {
	if os.Getenv("TF_ACC") != "1" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	var data map[string]any
	requests := 0
	transport := basicAuthTransport(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatal("wrong API authentication")
		}
		switch request.Method {
		case http.MethodGet:
			response := cloneMap(data)
			delete(response, "secret")
			return apiResponse(request, http.StatusOK, response)
		case http.MethodDelete:
			data = nil
			return apiResponse(request, http.StatusOK, map[string]any{"message": "Deleted"})
		case http.MethodPost, http.MethodPut:
			body := requestJSON(request)
			requests++
			expected := any(nil)
			if requests == 1 {
				expected = map[string]any{"username": "initial", "password": ""}
			}
			if requests == 2 {
				expected = map[string]any{"username": "rotated", "password": ""}
			}
			value, present := body["basic_auth"]
			if !present || !reflect.DeepEqual(value, expected) {
				t.Fatal("wrong Basic Auth request during apply")
			}
			if data == nil {
				data = map[string]any{"id": testWebhookID, "scope": "route", "route_id": testRouteID, "route_ids": []any{testRouteID}, "project_ids": []any{}, "delivery_mode_filter": "all", "secret": "whsec_test", "last_called_at": nil, "created_at": "2026-10-02T00:00:00Z", "updated_at": "2026-10-02T00:00:00Z"}
			}
			for key, value := range body {
				if key != "basic_auth" {
					data[key] = value
				}
			}
			data["has_basic_auth"] = value != nil
			response := cloneMap(data)
			status := http.StatusCreated
			if request.Method == http.MethodPut {
				delete(response, "secret")
				status = http.StatusOK
			}
			return apiResponse(request, status, map[string]any{"data": response, "message": "Success"})
		default:
			t.Fatal("unexpected acceptance request")
		}
		return nil, nil
	})
	factories := map[string]func() (tfprotov6.ProviderServer, error){"lettermint": providerserver.NewProtocol6WithError(&lettermintProvider{version: "test", clientFactory: func(token string) (*lettermint.APIClient, error) {
		return lettermint.NewAPI(token, lettermint.WithBaseURL("https://api.example.test/v1"), lettermint.WithHTTPClient(&http.Client{Transport: transport}))
	}})}
	config := func(auth string, version string) string {
		return `provider "lettermint" { team_token = "test-token" }
resource "lettermint_webhook" "test" {
  route_id = "` + testRouteID + `"
  name = "Delivery events"
  url = "https://example.com/webhook"
  events = ["message.delivered"]
  basic_auth = ` + auth + `
  basic_auth_version = ` + version + `
}`
	}
	check := func(flag string) testresource.TestCheckFunc {
		return testresource.ComposeAggregateTestCheckFunc(
			testresource.TestCheckResourceAttr("lettermint_webhook.test", "has_basic_auth", flag),
			testresource.TestCheckNoResourceAttr("lettermint_webhook.test", "basic_auth.username"),
			testresource.TestCheckNoResourceAttr("lettermint_webhook.test", "basic_auth.password"),
			testresource.TestCheckResourceAttr("lettermint_webhook.test", "secret", "whsec_test"),
		)
	}
	testresource.Test(t, testresource.TestCase{
		ProtoV6ProviderFactories: factories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_14_0)},
		CheckDestroy: func(*terraform.State) error {
			if data != nil || requests != 3 {
				t.Fatal("acceptance lifecycle was not complete")
			}
			return nil
		},
		Steps: []testresource.TestStep{
			{Config: config(`{ username = "initial", password = "" }`, "1"), Check: check("true")},
			{Config: config(`{ username = "rotated", password = "" }`, "2"), Check: check("true")},
			{Config: config("null", "3"), Check: check("false")},
		},
	})
}
