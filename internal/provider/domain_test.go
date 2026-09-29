package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lettermint/lettermint-go/v2"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestDomainModelPreservesDNSRecordsWhenRelationshipIsOmitted(t *testing.T) {
	t.Parallel()

	previous := domainResourceModel{DNSRecords: types.ListValueMust(
		domainDNSRecordObjectType(),
		[]attr.Value{domainDNSRecordValue(lettermint.DomainDnsRecordData{ID: "record-1"})},
	)}
	actual := domainModelFromAPI(lettermint.DomainData{ID: "domain-1", Domain: "example.com"}, previous)

	if domainDNSRecordID(t, actual.DNSRecords) != "record-1" {
		t.Fatalf("DNS records were not preserved: %#v", actual.DNSRecords)
	}
}

func TestDomainModelClearsDNSRecordsForExplicitEmptyRelationship(t *testing.T) {
	t.Parallel()

	previous := domainResourceModel{DNSRecords: types.ListValueMust(
		domainDNSRecordObjectType(),
		[]attr.Value{domainDNSRecordValue(lettermint.DomainDnsRecordData{ID: "record-1"})},
	)}
	actual := domainModelFromAPI(lettermint.DomainData{ID: "domain-1", Domain: "example.com", DNSRecords: []lettermint.DomainDnsRecordData{}}, previous)

	if actual.DNSRecords.IsNull() || len(actual.DNSRecords.Elements()) != 0 {
		t.Fatalf("DNS records were not cleared: %#v", actual.DNSRecords)
	}
}

func domainDNSRecordID(t *testing.T, records types.List) string {
	t.Helper()

	if records.IsNull() || records.IsUnknown() || len(records.Elements()) != 1 {
		t.Fatalf("DNS records = %#v, want one known record", records)
	}
	record, ok := records.Elements()[0].(types.Object)
	if !ok {
		t.Fatalf("DNS record type = %T, want types.Object", records.Elements()[0])
	}
	id, ok := record.Attributes()["id"].(types.String)
	if !ok {
		t.Fatalf("DNS record ID type = %T, want types.String", record.Attributes()["id"])
	}
	return id.ValueString()
}

func TestDomainCreateResponseDNSRecords(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name         string
		response     string
		wantNull     bool
		wantRecordID string
	}{
		{
			name: "with DNS records",
			response: `{
				"id":"domain-1","domain":"example.com","dkim_mode":"managed_cname","rotation_ready":false,
				"status_changed_at":null,"created_at":"2026-09-29T00:00:00Z",
				"dns_records":[{"id":"record-1","type":"CNAME","hostname":"lm1._domainkey","fqdn":"lm1._domainkey.example.com","content":"lm1.example.dkim.lettermint.co","status":"pending","purpose":"dkim_primary","verification_scope":"required","required_for_verification":true,"verified_at":null,"last_checked_at":null}]
			}`,
			wantRecordID: "record-1",
		},
		{
			name:     "without optional DNS records",
			response: `{"id":"domain-1","domain":"example.com","dkim_mode":"managed_cname","rotation_ready":false,"status_changed_at":null,"created_at":"2026-09-29T00:00:00Z"}`,
			wantNull: true,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if request.Method != http.MethodPost || request.URL.EscapedPath() != "/v1/domains" || request.URL.RawQuery != "" {
					t.Errorf("request = %s %s?%s", request.Method, request.URL.EscapedPath(), request.URL.RawQuery)
				}
				if got := request.Header.Get("Authorization"); got != "Bearer team-secret" {
					t.Errorf("Authorization = %q", got)
				}
				var body map[string]any
				if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
					t.Errorf("decode body: %v", err)
				}
				if len(body) != 1 || body["domain"] != "example.com" {
					t.Errorf("body = %#v", body)
				}
				return &http.Response{
					StatusCode: http.StatusCreated,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(testCase.response)),
					Request:    request,
				}, nil
			})}

			api, err := lettermint.NewAPI("team-secret", lettermint.WithBaseURL("https://api.example.test/v1"), lettermint.WithHTTPClient(client))
			if err != nil {
				t.Fatalf("NewAPI() error = %v", err)
			}
			underTest := &domainResource{client: &clientData{API: api}}

			var schemaResponse resource.SchemaResponse
			underTest.Schema(context.Background(), resource.SchemaRequest{}, &schemaResponse)
			plan := tfsdk.Plan{Schema: schemaResponse.Schema}
			planDiagnostics := plan.Set(context.Background(), &domainResourceModel{
				ID:              types.StringUnknown(),
				Domain:          types.StringValue("example.com"),
				DKIMMode:        types.StringUnknown(),
				RotationReady:   types.BoolUnknown(),
				StatusChangedAt: types.StringUnknown(),
				DNSRecords:      types.ListUnknown(domainDNSRecordObjectType()),
				CreatedAt:       types.StringUnknown(),
			})
			if planDiagnostics.HasError() {
				t.Fatalf("plan diagnostics = %v", planDiagnostics)
			}
			response := resource.CreateResponse{State: tfsdk.State{Schema: schemaResponse.Schema}}
			underTest.Create(context.Background(), resource.CreateRequest{Plan: plan}, &response)
			if response.Diagnostics.HasError() {
				t.Fatalf("create diagnostics = %v", response.Diagnostics)
			}

			var state domainResourceModel
			stateDiagnostics := response.State.Get(context.Background(), &state)
			if stateDiagnostics.HasError() {
				t.Fatalf("state diagnostics = %v", stateDiagnostics)
			}
			if testCase.wantNull {
				if !state.DNSRecords.IsNull() {
					t.Fatalf("DNS records = %#v, want null", state.DNSRecords)
				}
				return
			}
			if domainDNSRecordID(t, state.DNSRecords) != testCase.wantRecordID {
				t.Fatalf("DNS records = %#v", state.DNSRecords)
			}
		})
	}
}
