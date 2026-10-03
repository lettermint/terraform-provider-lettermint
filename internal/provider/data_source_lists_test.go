package provider

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lettermint/lettermint-go/v3"
)

func dataSourceConfig(t *testing.T, schema datasourceschema.Schema, value any) tfsdk.Config {
	t.Helper()
	state := tfsdk.State{Schema: schema}
	diagnostics := state.Set(context.Background(), value)
	if diagnostics.HasError() {
		t.Fatalf("config diagnostics = %v", diagnostics)
	}
	return tfsdk.Config{Schema: schema, Raw: state.Raw}
}

func dataSourceClient(t *testing.T, transport roundTripFunc) *clientData {
	t.Helper()
	api, err := lettermint.New(
		lettermint.WithTeamToken("team-secret"),
		lettermint.WithBaseURL("https://api.example.test/v1"),
		lettermint.WithHTTPClient(&http.Client{Transport: transport}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return &clientData{Client: api}
}

func TestProjectsDataSourceUsesDocumentedQueriesAndPagination(t *testing.T) {
	t.Parallel()

	requestCount := 0
	client := dataSourceClient(t, func(request *http.Request) (*http.Response, error) {
		requestCount++
		if request.Method != http.MethodGet || request.URL.EscapedPath() != "/v1/projects" {
			t.Errorf("request = %s %s", request.Method, request.URL.EscapedPath())
		}
		if got := request.Header.Get("Authorization"); got != "Bearer team-secret" {
			t.Errorf("Authorization = %q", got)
		}
		expectedQuery := url.Values{
			"filter[search]": {"production"},
			"page[size]":     {"10"},
			"sort":           {"name,-created_at"},
		}
		if requestCount == 2 {
			expectedQuery["page[cursor]"] = []string{"next-page"}
		}
		if !reflect.DeepEqual(request.URL.Query(), expectedQuery) {
			t.Errorf("query = %#v, want %#v", request.URL.Query(), expectedQuery)
		}
		body := `{"data":[{"id":"project-1","name":"One","smtp_enabled":true,"routes_count":1,"domains_count":1,"last_28_days":{},"created_at":"2026-09-29T00:00:00Z","updated_at":"2026-09-29T00:00:00Z"}],"path":null,"per_page":10,"next_cursor":"next-page","next_page_url":null,"prev_cursor":null,"prev_page_url":null}`
		if requestCount == 2 {
			body = `{"data":[{"id":"project-2","name":"Two","smtp_enabled":false,"routes_count":0,"domains_count":0,"last_28_days":{},"created_at":"2026-09-29T00:00:00Z","updated_at":"2026-09-29T00:00:00Z"}],"path":null,"per_page":10,"next_cursor":null,"next_page_url":null,"prev_cursor":"previous-page","prev_page_url":null}`
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    request,
		}, nil
	})

	underTest := &projectsDataSource{client: client}
	var schemaResponse datasource.SchemaResponse
	underTest.Schema(context.Background(), datasource.SchemaRequest{}, &schemaResponse)
	config := projectsDataSourceModel{
		Search:   types.StringValue("production"),
		Sort:     types.ListValueMust(types.StringType, []attr.Value{types.StringValue("name"), types.StringValue("-created_at")}),
		PageSize: types.Int64Value(10),
		Projects: nil,
	}
	response := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResponse.Schema}}
	underTest.Read(context.Background(), datasource.ReadRequest{Config: dataSourceConfig(t, schemaResponse.Schema, &config)}, &response)
	if response.Diagnostics.HasError() {
		t.Fatalf("read diagnostics = %v", response.Diagnostics)
	}
	if requestCount != 2 {
		t.Fatalf("request count = %d, want 2", requestCount)
	}
	var state projectsDataSourceModel
	if diagnostics := response.State.Get(context.Background(), &state); diagnostics.HasError() {
		t.Fatalf("state diagnostics = %v", diagnostics)
	}
	if len(state.Projects) != 2 || state.Projects[0].ID.ValueString() != "project-1" || state.Projects[1].ID.ValueString() != "project-2" {
		t.Fatalf("projects = %#v", state.Projects)
	}
}

func TestWebhooksDataSourceUsesOnlyDocumentedFirstPageQuery(t *testing.T) {
	t.Parallel()

	requestCount := 0
	client := dataSourceClient(t, func(request *http.Request) (*http.Response, error) {
		requestCount++
		if request.Method != http.MethodGet || request.URL.EscapedPath() != "/v1/webhooks" {
			t.Errorf("request = %s %s", request.Method, request.URL.EscapedPath())
		}
		if got := request.Header.Get("Authorization"); got != "Bearer team-secret" {
			t.Errorf("Authorization = %q", got)
		}
		// The SDK sends booleans as 1/0 and leaves out an empty sort list.
		expectedQuery := url.Values{
			"filter[enabled]":  {"0"},
			"filter[event]":    {"message.delivered"},
			"filter[route_id]": {testRouteID},
			"filter[search]":   {"delivery"},
		}
		if !reflect.DeepEqual(request.URL.Query(), expectedQuery) {
			t.Errorf("query = %#v, want %#v", request.URL.Query(), expectedQuery)
		}
		body := `{"data":[{"id":"webhook-1","scope":"route","project_ids":[],"route_ids":["` + testRouteID + `"],"route_id":"` + testRouteID + `","name":"Delivery","url":"https://example.com/hook","events":["message.delivered"],"enabled":false,"last_called_at":null,"created_at":"2026-09-29T00:00:00Z","updated_at":"2026-09-29T00:00:00Z"}],"path":null,"per_page":30,"next_cursor":"not-requestable","next_page_url":null,"prev_cursor":null,"prev_page_url":null}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    request,
		}, nil
	})

	underTest := &webhooksDataSource{client: client}
	var schemaResponse datasource.SchemaResponse
	underTest.Schema(context.Background(), datasource.SchemaRequest{}, &schemaResponse)
	config := webhooksDataSourceModel{
		RouteID:  types.StringValue(testRouteID),
		Enabled:  types.BoolValue(false),
		Event:    types.StringValue("message.delivered"),
		Search:   types.StringValue("delivery"),
		Sort:     types.ListValueMust(types.StringType, []attr.Value{}),
		Webhooks: nil,
	}
	response := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResponse.Schema}}
	underTest.Read(context.Background(), datasource.ReadRequest{Config: dataSourceConfig(t, schemaResponse.Schema, &config)}, &response)
	if response.Diagnostics.HasError() {
		t.Fatalf("read diagnostics = %v", response.Diagnostics)
	}
	if requestCount != 1 {
		t.Fatalf("request count = %d, want 1", requestCount)
	}
	var state webhooksDataSourceModel
	if diagnostics := response.State.Get(context.Background(), &state); diagnostics.HasError() {
		t.Fatalf("state diagnostics = %v", diagnostics)
	}
	if len(state.Webhooks) != 1 || state.Webhooks[0].ID.ValueString() != "webhook-1" {
		t.Fatalf("webhooks = %#v", state.Webhooks)
	}
}
