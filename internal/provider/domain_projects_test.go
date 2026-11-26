package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lettermint/lettermint-go/v3"
)

const (
	testProjectA = "01a03585-0000-7000-8000-00000000000a"
	testProjectB = "01a03585-0000-7000-8000-00000000000b"
)

type domainAPICall struct {
	method string
	path   string
	query  string
	body   map[string]any
}

// domainProjectsTestResource returns a domain resource whose API answers each
// request with the next response, and records the requests it receives.
func domainProjectsTestResource(t *testing.T, responses ...string) (*domainResource, *[]domainAPICall) {
	t.Helper()

	calls := &[]domainAPICall{}
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		call := domainAPICall{method: request.Method, path: request.URL.EscapedPath(), query: request.URL.RawQuery}
		if request.Body != nil {
			raw, _ := io.ReadAll(request.Body)
			if len(raw) > 0 {
				if err := json.Unmarshal(raw, &call.body); err != nil {
					t.Errorf("decode body %q: %v", raw, err)
				}
			}
		}
		*calls = append(*calls, call)
		if len(*calls) > len(responses) {
			t.Fatalf("unexpected request %s %s?%s", call.method, call.path, call.query)
		}
		status := http.StatusOK
		if call.method == http.MethodPost {
			status = http.StatusCreated
		}
		return &http.Response{
			StatusCode: status,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(responses[len(*calls)-1])),
			Request:    request,
		}, nil
	})}

	api, err := lettermint.New(lettermint.WithTeamToken("team-secret"), lettermint.WithBaseURL("https://api.example.test/v1"), lettermint.WithHTTPClient(client))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return &domainResource{client: &clientData{Client: api}}, calls
}

func domainProjectsTestSchema(t *testing.T) resource.SchemaResponse {
	t.Helper()

	var response resource.SchemaResponse
	(&domainResource{}).Schema(context.Background(), resource.SchemaRequest{}, &response)
	if response.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics = %v", response.Diagnostics)
	}
	return response
}

func projectIDSet(ids ...string) types.Set {
	elements := make([]attr.Value, 0, len(ids))
	for _, id := range ids {
		elements = append(elements, types.StringValue(id))
	}
	return types.SetValueMust(types.StringType, elements)
}

func projectIDsOf(t *testing.T, value types.Set) []string {
	t.Helper()

	var ids []string
	if diags := value.ElementsAs(context.Background(), &ids, false); diags.HasError() {
		t.Fatalf("project IDs diagnostics = %v", diags)
	}
	slices.Sort(ids)
	return ids
}

func requestProjectIDs(t *testing.T, call domainAPICall) []string {
	t.Helper()

	raw, ok := call.body["project_ids"].([]any)
	if !ok || len(call.body) != 1 {
		t.Fatalf("body = %#v, want only a project_ids list", call.body)
	}
	ids := make([]string, 0, len(raw))
	for _, id := range raw {
		ids = append(ids, id.(string))
	}
	slices.Sort(ids)
	return ids
}

func domainResponse(projects string) string {
	body := `{"id":"domain-1","domain":"example.com","dkim_mode":"managed_cname","rotation_ready":false,"status_changed_at":null,"created_at":"2026-09-29T00:00:00Z"`
	if projects != "" {
		body += `,"projects":` + projects
	}
	return body + "}"
}

func domainMutationResponse(projects string) string {
	return `{"message":"Domain projects updated successfully.","data":` + domainResponse(projects) + `}`
}

func knownDomainState(projectIDs types.Set) domainResourceModel {
	return domainResourceModel{
		ID:              types.StringValue("domain-1"),
		Domain:          types.StringValue("example.com"),
		DKIMMode:        types.StringValue("managed_cname"),
		RotationReady:   types.BoolValue(false),
		StatusChangedAt: types.StringNull(),
		DNSRecords: types.ListValueMust(domainDNSRecordObjectType(), []attr.Value{
			domainDNSRecordValue(lettermint.DomainDnsRecordData{ID: "record-1"}),
		}),
		ProjectIDs: projectIDs,
		CreatedAt:  types.StringValue("2026-09-29T00:00:00Z"),
	}
}

func TestDomainCreateAssignsProjects(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name       string
		projectIDs types.Set
		responses  []string
		wantCalls  int
		wantIDs    []string
		wantNull   bool
	}{
		{
			name:       "without project_ids",
			projectIDs: types.SetNull(types.StringType),
			responses:  []string{domainResponse("")},
			wantCalls:  1,
			wantNull:   true,
		},
		{
			name:       "with an empty set",
			projectIDs: projectIDSet(),
			responses:  []string{domainResponse("")},
			wantCalls:  1,
			wantIDs:    nil,
		},
		{
			name:       "with projects",
			projectIDs: projectIDSet(testProjectB, testProjectA),
			responses: []string{
				domainResponse(""),
				domainMutationResponse(`[{"id":"` + testProjectA + `","name":"A"},{"id":"` + testProjectB + `","name":"B"}]`),
			},
			wantCalls: 2,
			wantIDs:   []string{testProjectA, testProjectB},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			underTest, calls := domainProjectsTestResource(t, testCase.responses...)
			schemaResponse := domainProjectsTestSchema(t)
			plan := tfsdk.Plan{Schema: schemaResponse.Schema}
			if diags := plan.Set(context.Background(), &domainResourceModel{
				ID:              types.StringUnknown(),
				Domain:          types.StringValue("example.com"),
				DKIMMode:        types.StringUnknown(),
				RotationReady:   types.BoolUnknown(),
				StatusChangedAt: types.StringUnknown(),
				DNSRecords:      types.ListUnknown(domainDNSRecordObjectType()),
				ProjectIDs:      testCase.projectIDs,
				CreatedAt:       types.StringUnknown(),
			}); diags.HasError() {
				t.Fatalf("plan diagnostics = %v", diags)
			}

			response := resource.CreateResponse{State: tfsdk.State{Schema: schemaResponse.Schema}}
			underTest.Create(context.Background(), resource.CreateRequest{Plan: plan}, &response)
			if response.Diagnostics.HasError() {
				t.Fatalf("create diagnostics = %v", response.Diagnostics)
			}
			if len(*calls) != testCase.wantCalls {
				t.Fatalf("calls = %#v, want %d", *calls, testCase.wantCalls)
			}
			if testCase.wantCalls == 2 {
				call := (*calls)[1]
				if call.method != http.MethodPut || call.path != "/v1/domains/domain-1/projects" {
					t.Fatalf("assignment request = %s %s", call.method, call.path)
				}
				if got := requestProjectIDs(t, call); !slices.Equal(got, testCase.wantIDs) {
					t.Fatalf("requested project IDs = %v, want %v", got, testCase.wantIDs)
				}
			}

			var state domainResourceModel
			if diags := response.State.Get(context.Background(), &state); diags.HasError() {
				t.Fatalf("state diagnostics = %v", diags)
			}
			if testCase.wantNull {
				if !state.ProjectIDs.IsNull() {
					t.Fatalf("project IDs = %#v, want null", state.ProjectIDs)
				}
				return
			}
			if got := projectIDsOf(t, state.ProjectIDs); !slices.Equal(got, testCase.wantIDs) {
				t.Fatalf("project IDs = %v, want %v", got, testCase.wantIDs)
			}
		})
	}
}

func TestDomainCreateKeepsDomainWhenAssignmentFails(t *testing.T) {
	t.Parallel()

	api, err := lettermint.New(lettermint.WithTeamToken("team-secret"), lettermint.WithBaseURL("https://api.example.test/v1"), lettermint.WithHTTPClient(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		status, body := http.StatusCreated, domainResponse("")
		if request.Method == http.MethodPut {
			status, body = http.StatusUnprocessableEntity, `{"message":"One or more project IDs do not belong to this team.","error_code":"INVALID_PROJECT_IDS"}`
		}
		return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
	})}))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	underTest := &domainResource{client: &clientData{Client: api}}

	schemaResponse := domainProjectsTestSchema(t)
	plan := tfsdk.Plan{Schema: schemaResponse.Schema}
	if diags := plan.Set(context.Background(), &domainResourceModel{
		ID:              types.StringUnknown(),
		Domain:          types.StringValue("example.com"),
		DKIMMode:        types.StringUnknown(),
		RotationReady:   types.BoolUnknown(),
		StatusChangedAt: types.StringUnknown(),
		DNSRecords:      types.ListUnknown(domainDNSRecordObjectType()),
		ProjectIDs:      projectIDSet(testProjectA),
		CreatedAt:       types.StringUnknown(),
	}); diags.HasError() {
		t.Fatalf("plan diagnostics = %v", diags)
	}

	response := resource.CreateResponse{State: tfsdk.State{Schema: schemaResponse.Schema}}
	underTest.Create(context.Background(), resource.CreateRequest{Plan: plan}, &response)
	if !response.Diagnostics.HasError() {
		t.Fatal("create succeeded, want an assignment error")
	}

	var state domainResourceModel
	if diags := response.State.Get(context.Background(), &state); diags.HasError() {
		t.Fatalf("state diagnostics = %v", diags)
	}
	if state.ID.ValueString() != "domain-1" {
		t.Fatalf("state ID = %#v, want the created domain", state.ID)
	}
	if ids := projectIDsOf(t, state.ProjectIDs); len(ids) != 0 {
		t.Fatalf("project IDs = %v, want the empty set of a new domain", ids)
	}
}

func TestDomainReadProjects(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name      string
		previous  types.Set
		response  string
		wantQuery string
		wantIDs   []string
		wantNull  bool
	}{
		{
			name:     "not managed",
			previous: types.SetNull(types.StringType),
			response: domainResponse(""),
			wantNull: true,
		},
		{
			name:      "assigned in the dashboard",
			previous:  projectIDSet(testProjectA),
			response:  domainResponse(`[{"id":"` + testProjectA + `","name":"A"},{"id":"` + testProjectB + `","name":"B"}]`),
			wantQuery: "include=projects",
			wantIDs:   []string{testProjectA, testProjectB},
		},
		{
			name:      "unrestricted",
			previous:  projectIDSet(testProjectA),
			response:  domainResponse(`[]`),
			wantQuery: "include=projects",
			wantIDs:   nil,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			underTest, calls := domainProjectsTestResource(t, testCase.response)
			schemaResponse := domainProjectsTestSchema(t)
			state := tfsdk.State{Schema: schemaResponse.Schema}
			previous := knownDomainState(testCase.previous)
			if diags := state.Set(context.Background(), &previous); diags.HasError() {
				t.Fatalf("state diagnostics = %v", diags)
			}

			response := resource.ReadResponse{State: state}
			underTest.Read(context.Background(), resource.ReadRequest{State: state}, &response)
			if response.Diagnostics.HasError() {
				t.Fatalf("read diagnostics = %v", response.Diagnostics)
			}
			if call := (*calls)[0]; call.method != http.MethodGet || call.path != "/v1/domains/domain-1" || call.query != testCase.wantQuery {
				t.Fatalf("request = %s %s?%s, want query %q", call.method, call.path, call.query, testCase.wantQuery)
			}

			var actual domainResourceModel
			if diags := response.State.Get(context.Background(), &actual); diags.HasError() {
				t.Fatalf("state diagnostics = %v", diags)
			}
			if domainDNSRecordID(t, actual.DNSRecords) != "record-1" {
				t.Fatalf("DNS records were not preserved: %#v", actual.DNSRecords)
			}
			if testCase.wantNull {
				if !actual.ProjectIDs.IsNull() {
					t.Fatalf("project IDs = %#v, want null", actual.ProjectIDs)
				}
				return
			}
			if got := projectIDsOf(t, actual.ProjectIDs); !slices.Equal(got, testCase.wantIDs) {
				t.Fatalf("project IDs = %v, want %v", got, testCase.wantIDs)
			}
		})
	}
}

func TestDomainUpdateProjects(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name        string
		previous    types.Set
		planned     types.Set
		response    string
		wantRequest bool
		wantIDs     []string
		wantNull    bool
	}{
		{
			name:        "replace projects",
			previous:    projectIDSet(testProjectA),
			planned:     projectIDSet(testProjectB),
			response:    domainMutationResponse(`[{"id":"` + testProjectB + `","name":"B"}]`),
			wantRequest: true,
			wantIDs:     []string{testProjectB},
		},
		{
			name:        "make the domain unrestricted",
			previous:    projectIDSet(testProjectA),
			planned:     projectIDSet(),
			response:    domainMutationResponse(`[]`),
			wantRequest: true,
			wantIDs:     []string{},
		},
		{
			name:        "start managing an imported domain",
			previous:    types.SetNull(types.StringType),
			planned:     projectIDSet(testProjectA),
			response:    domainMutationResponse(`[{"id":"` + testProjectA + `","name":"A"}]`),
			wantRequest: true,
			wantIDs:     []string{testProjectA},
		},
		{
			name:     "stop managing projects",
			previous: projectIDSet(testProjectA),
			planned:  types.SetNull(types.StringType),
			wantNull: true,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var responses []string
			if testCase.response != "" {
				responses = append(responses, testCase.response)
			}
			underTest, calls := domainProjectsTestResource(t, responses...)
			schemaResponse := domainProjectsTestSchema(t)

			state := tfsdk.State{Schema: schemaResponse.Schema}
			previous := knownDomainState(testCase.previous)
			if diags := state.Set(context.Background(), &previous); diags.HasError() {
				t.Fatalf("state diagnostics = %v", diags)
			}
			planned := knownDomainState(testCase.planned)
			planned.DKIMMode = types.StringUnknown()
			planned.RotationReady = types.BoolUnknown()
			planned.StatusChangedAt = types.StringUnknown()
			plan := tfsdk.Plan{Schema: schemaResponse.Schema}
			if diags := plan.Set(context.Background(), &planned); diags.HasError() {
				t.Fatalf("plan diagnostics = %v", diags)
			}

			response := resource.UpdateResponse{State: tfsdk.State{Schema: schemaResponse.Schema}}
			underTest.Update(context.Background(), resource.UpdateRequest{Plan: plan, State: state}, &response)
			if response.Diagnostics.HasError() {
				t.Fatalf("update diagnostics = %v", response.Diagnostics)
			}

			if !testCase.wantRequest {
				if len(*calls) != 0 {
					t.Fatalf("calls = %#v, want none", *calls)
				}
			} else {
				if len(*calls) != 1 {
					t.Fatalf("calls = %#v, want one", *calls)
				}
				call := (*calls)[0]
				if call.method != http.MethodPut || call.path != "/v1/domains/domain-1/projects" {
					t.Fatalf("request = %s %s", call.method, call.path)
				}
				// An empty set must reach the API as an empty list, not as null.
				if got := requestProjectIDs(t, call); !slices.Equal(got, testCase.wantIDs) {
					t.Fatalf("requested project IDs = %v, want %v", got, testCase.wantIDs)
				}
			}

			var actual domainResourceModel
			if diags := response.State.Get(context.Background(), &actual); diags.HasError() {
				t.Fatalf("state diagnostics = %v", diags)
			}
			if actual.DKIMMode.IsUnknown() || actual.RotationReady.IsUnknown() || actual.StatusChangedAt.IsUnknown() {
				t.Fatalf("state has unknown values: %#v", actual)
			}
			if domainDNSRecordID(t, actual.DNSRecords) != "record-1" {
				t.Fatalf("DNS records were not preserved: %#v", actual.DNSRecords)
			}
			if testCase.wantNull {
				if !actual.ProjectIDs.IsNull() {
					t.Fatalf("project IDs = %#v, want null", actual.ProjectIDs)
				}
				return
			}
			if got := projectIDsOf(t, actual.ProjectIDs); !slices.Equal(got, testCase.wantIDs) {
				t.Fatalf("project IDs = %v, want %v", got, testCase.wantIDs)
			}
		})
	}
}
