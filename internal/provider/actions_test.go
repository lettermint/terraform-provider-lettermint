package provider

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/action"
	actionschema "github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/lettermint/lettermint-go/v2"
)

func actionConfig(ctx context.Context, schema actionschema.Schema, values map[string]string) tfsdk.Config {
	rawValues := make(map[string]tftypes.Value, len(values))
	for name, value := range values {
		rawValues[name] = tftypes.NewValue(tftypes.String, value)
	}
	return tfsdk.Config{
		Schema: schema,
		Raw:    tftypes.NewValue(schema.Type().TerraformType(ctx), rawValues),
	}
}

func TestVerificationActionsUseDocumentedRequests(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		path         string
		responseBody string
		values       map[string]string
		invoke       func(context.Context, *clientData, tfsdk.Config, *action.InvokeResponse)
		wantProgress string
	}{
		{
			name:         "verify all domain DNS records",
			path:         "/v1/domains/domain-1/dns-records/verify",
			responseBody: `{"message":"DNS verification completed.","recommended_failed_records":[]}`,
			values:       map[string]string{"domain_id": "domain-1"},
			wantProgress: "DNS verification completed.",
			invoke: func(ctx context.Context, client *clientData, config tfsdk.Config, response *action.InvokeResponse) {
				underTest := &verifyDomainDNSAction{client: client}
				underTest.Invoke(ctx, action.InvokeRequest{Config: config}, response)
			},
		},
		{
			name:         "verify one domain DNS record",
			path:         "/v1/domains/domain-1/dns-records/record-1/verify",
			responseBody: `{"message":"DNS record verified successfully."}`,
			values:       map[string]string{"domain_id": "domain-1", "record_id": "record-1"},
			wantProgress: "DNS record verified successfully.",
			invoke: func(ctx context.Context, client *clientData, config tfsdk.Config, response *action.InvokeResponse) {
				underTest := &verifyDomainDNSRecordAction{client: client}
				underTest.Invoke(ctx, action.InvokeRequest{Config: config}, response)
			},
		},
		{
			name:         "verify route inbound domain",
			path:         "/v1/routes/route-1/verify-inbound-domain",
			responseBody: `{"data":{"verified":true,"message":"Inbound domain verified successfully."}}`,
			values:       map[string]string{"route_id": "route-1"},
			wantProgress: "Inbound domain verified successfully.",
			invoke: func(ctx context.Context, client *clientData, config tfsdk.Config, response *action.InvokeResponse) {
				underTest := &verifyRouteInboundDomainAction{client: client}
				underTest.Invoke(ctx, action.InvokeRequest{Config: config}, response)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if request.Method != http.MethodPost || request.URL.EscapedPath() != testCase.path || request.URL.RawQuery != "" {
					t.Errorf("request = %s %s?%s", request.Method, request.URL.EscapedPath(), request.URL.RawQuery)
				}
				if got := request.Header.Get("Authorization"); got != "Bearer team-secret" {
					t.Errorf("Authorization = %q", got)
				}
				if request.Body != nil {
					body, err := io.ReadAll(request.Body)
					if err != nil {
						t.Errorf("read body: %v", err)
					}
					if len(body) != 0 {
						t.Errorf("body = %q", body)
					}
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(testCase.responseBody)),
					Request:    request,
				}, nil
			})}
			api, err := lettermint.NewAPI("team-secret", lettermint.WithBaseURL("https://api.example.test/v1"), lettermint.WithHTTPClient(client))
			if err != nil {
				t.Fatalf("NewAPI() error = %v", err)
			}

			var schema actionschema.Schema
			switch testCase.name {
			case "verify all domain DNS records":
				var schemaResponse action.SchemaResponse
				(&verifyDomainDNSAction{}).Schema(ctx, action.SchemaRequest{}, &schemaResponse)
				schema = schemaResponse.Schema
			case "verify one domain DNS record":
				var schemaResponse action.SchemaResponse
				(&verifyDomainDNSRecordAction{}).Schema(ctx, action.SchemaRequest{}, &schemaResponse)
				schema = schemaResponse.Schema
			case "verify route inbound domain":
				var schemaResponse action.SchemaResponse
				(&verifyRouteInboundDomainAction{}).Schema(ctx, action.SchemaRequest{}, &schemaResponse)
				schema = schemaResponse.Schema
			}

			var progress string
			response := &action.InvokeResponse{SendProgress: func(event action.InvokeProgressEvent) { progress = event.Message }}
			testCase.invoke(ctx, &clientData{API: api}, actionConfig(ctx, schema, testCase.values), response)
			if response.Diagnostics.HasError() {
				t.Fatalf("invoke diagnostics = %v", response.Diagnostics)
			}
			if progress != testCase.wantProgress {
				t.Fatalf("progress = %q, want %q", progress, testCase.wantProgress)
			}
		})
	}
}
