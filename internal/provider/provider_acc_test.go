package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/lettermint/lettermint-go/v2"
)

const (
	testProjectID = "00000000-0000-4000-8000-000000000001"
	testDomainID  = "00000000-0000-4000-8000-000000000002"
	testRouteID   = "00000000-0000-4000-8000-000000000003"
	testWebhookID = "00000000-0000-4000-8000-000000000004"
	testRecordID  = "00000000-0000-4000-8000-000000000005"
)

type acceptanceAPI struct {
	t                   *testing.T
	mu                  sync.Mutex
	project             map[string]any
	domain              map[string]any
	route               map[string]any
	webhook             map[string]any
	verificationInvokes map[string]int
}

func newAcceptanceAPI(t *testing.T) *acceptanceAPI {
	return &acceptanceAPI{t: t, verificationInvokes: map[string]int{}}
}

func (a *acceptanceAPI) RoundTrip(request *http.Request) (*http.Response, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if request.Header.Get("Authorization") != "Bearer test-token" {
		return apiResponse(request, http.StatusUnauthorized, map[string]any{"message": "Unauthorized."})
	}
	if request.URL.RawQuery != "" {
		a.t.Errorf("unexpected query for %s %s: %s", request.Method, request.URL.EscapedPath(), request.URL.RawQuery)
	}
	path := request.URL.EscapedPath()
	switch {
	case request.Method == http.MethodPost && path == "/v1/projects":
		body := requestJSON(request)
		a.expectBody(body, map[string]any{
			"name": "Acceptance project", "smtp_enabled": true, "initial_routes": "both", "short_token": false,
		})
		a.project = map[string]any{
			"id": testProjectID, "name": body["name"], "smtp_enabled": body["smtp_enabled"],
			"redact_email_content": false, "default_route_id": nil,
			"token_generated_at": "2026-09-29T00:00:00Z", "token_last_used_at": nil, "token_last_used_ip": nil,
			"created_at": "2026-09-29T00:00:00Z", "updated_at": "2026-09-29T00:00:00Z", "delivery_mode": "live",
		}
		return apiResponse(request, http.StatusCreated, map[string]any{"data": a.project, "message": "Project created successfully.", "api_token": "lm_project_test"})
	case request.Method == http.MethodGet && path == "/v1/projects/"+testProjectID:
		return a.read(request, a.project)
	case request.Method == http.MethodPut && path == "/v1/projects/"+testProjectID:
		if a.project == nil {
			return apiResponse(request, http.StatusNotFound, map[string]any{"message": "Not found."})
		}
		body := requestJSON(request)
		if a.project["name"] == "Remote drift" {
			a.expectBody(body, map[string]any{"name": "Acceptance project", "smtp_enabled": true, "redact_email_content": true})
		} else {
			a.expectBody(body, map[string]any{"redact_email_content": true})
		}
		for key, value := range body {
			a.project[key] = value
		}
		a.project["updated_at"] = "2026-09-29T00:01:00Z"
		return apiResponse(request, http.StatusOK, map[string]any{"data": a.project, "message": "Project updated successfully."})
	case request.Method == http.MethodDelete && path == "/v1/projects/"+testProjectID:
		a.project = nil
		return apiResponse(request, http.StatusOK, map[string]any{"message": "Project deleted successfully."})

	case request.Method == http.MethodPost && path == "/v1/domains":
		body := requestJSON(request)
		a.expectBody(body, map[string]any{"domain": "example.com"})
		a.domain = map[string]any{
			"id": testDomainID, "domain": body["domain"], "dkim_mode": "managed_cname", "rotation_ready": false,
			"status_changed_at": nil, "created_at": "2026-09-29T00:00:00Z",
			"dns_records": []any{map[string]any{
				"id": testRecordID, "type": "CNAME", "hostname": "lm1._domainkey", "fqdn": "lm1._domainkey.example.com",
				"content": "lm1.example.dkim.lettermint.co", "status": "pending", "purpose": "dkim_primary",
				"verification_scope": "required", "required_for_verification": true, "verified_at": nil, "last_checked_at": nil,
			}},
		}
		return apiResponse(request, http.StatusCreated, a.domain)
	case request.Method == http.MethodGet && path == "/v1/domains/"+testDomainID:
		if a.domain == nil {
			return apiResponse(request, http.StatusNotFound, map[string]any{"message": "Not found."})
		}
		response := cloneMap(a.domain)
		delete(response, "dns_records")
		return apiResponse(request, http.StatusOK, response)
	case request.Method == http.MethodDelete && path == "/v1/domains/"+testDomainID:
		a.domain = nil
		return apiResponse(request, http.StatusOK, map[string]any{"message": "Domain deleted successfully."})
	case request.Method == http.MethodPost && path == "/v1/domains/"+testDomainID+"/dns-records/verify":
		a.verificationInvokes["domain"]++
		return apiResponse(request, http.StatusOK, map[string]any{"message": "DNS verification completed.", "recommended_failed_records": []any{}})
	case request.Method == http.MethodPost && path == "/v1/domains/"+testDomainID+"/dns-records/"+testRecordID+"/verify":
		a.verificationInvokes["record"]++
		return apiResponse(request, http.StatusOK, map[string]any{"message": "DNS record verified successfully."})

	case request.Method == http.MethodPost && path == "/v1/projects/"+testProjectID+"/routes":
		body := requestJSON(request)
		a.expectBody(body, map[string]any{"name": "Support", "route_type": "inbound"})
		a.route = map[string]any{
			"id": testRouteID, "project_id": testProjectID, "slug": "support", "name": body["name"], "route_type": body["route_type"],
			"is_default": false, "inbound_address": "support@inbound.lettermint.co", "inbound_domain": nil,
			"inbound_domain_verified_at": nil, "inbound_spam_threshold": nil, "attachment_delivery": "inline",
			"settings": map[string]any{}, "created_at": "2026-09-29T00:00:00Z", "updated_at": "2026-09-29T00:00:00Z",
		}
		return apiResponse(request, http.StatusCreated, map[string]any{"data": a.route, "message": "Route created successfully."})
	case request.Method == http.MethodGet && path == "/v1/routes/"+testRouteID:
		return a.read(request, a.route)
	case request.Method == http.MethodPut && path == "/v1/routes/"+testRouteID:
		if a.route == nil {
			return apiResponse(request, http.StatusNotFound, map[string]any{"message": "Not found."})
		}
		body := requestJSON(request)
		a.expectBody(body, map[string]any{"inbound_settings": map[string]any{
			"inbound_domain": "support.example.com", "inbound_spam_threshold": float64(5), "attachment_delivery": "url",
		}})
		if value, ok := body["name"]; ok {
			a.route["name"] = value
		}
		if settings, ok := body["settings"].(map[string]any); ok {
			a.route["settings"] = settings
		}
		if inbound, ok := body["inbound_settings"].(map[string]any); ok {
			for _, key := range []string{"inbound_domain", "inbound_spam_threshold", "attachment_delivery"} {
				if value, exists := inbound[key]; exists {
					a.route[key] = value
				}
			}
		}
		a.route["updated_at"] = "2026-09-29T00:01:00Z"
		return apiResponse(request, http.StatusOK, map[string]any{"data": a.route, "message": "Route updated successfully."})
	case request.Method == http.MethodDelete && path == "/v1/routes/"+testRouteID:
		a.route = nil
		return apiResponse(request, http.StatusOK, map[string]any{"message": "Route deleted successfully."})
	case request.Method == http.MethodPost && path == "/v1/routes/"+testRouteID+"/verify-inbound-domain":
		a.verificationInvokes["route"]++
		return apiResponse(request, http.StatusOK, map[string]any{"data": map[string]any{"verified": true, "message": "Inbound domain verified successfully."}})

	case request.Method == http.MethodPost && path == "/v1/webhooks":
		body := requestJSON(request)
		a.expectBody(body, map[string]any{
			"route_id": testRouteID, "name": "Delivery events", "url": "https://example.com/webhooks/lettermint",
			"events": []any{"message.delivered"}, "enabled": true, "include_machine_events": false,
		})
		a.webhook = map[string]any{
			"id": testWebhookID, "scope": "route", "project_ids": []any{}, "route_ids": []any{testRouteID}, "route_id": body["route_id"],
			"name": body["name"], "url": body["url"], "events": body["events"], "enabled": body["enabled"],
			"include_machine_events": body["include_machine_events"], "secret": "whsec_test", "last_called_at": nil, "has_basic_auth": false,
			"created_at": "2026-09-29T00:00:00Z", "updated_at": "2026-09-29T00:00:00Z", "delivery_mode_filter": "all",
		}
		return apiResponse(request, http.StatusCreated, map[string]any{"data": a.webhook, "message": "Webhook created successfully."})
	case request.Method == http.MethodGet && path == "/v1/webhooks/"+testWebhookID:
		if a.webhook == nil {
			return apiResponse(request, http.StatusNotFound, map[string]any{"message": "Not found."})
		}
		response := cloneMap(a.webhook)
		delete(response, "secret")
		return apiResponse(request, http.StatusOK, response)
	case request.Method == http.MethodPut && path == "/v1/webhooks/"+testWebhookID:
		if a.webhook == nil {
			return apiResponse(request, http.StatusNotFound, map[string]any{"message": "Not found."})
		}
		for key, value := range requestJSON(request) {
			a.webhook[key] = value
		}
		a.webhook["updated_at"] = "2026-09-29T00:01:00Z"
		response := cloneMap(a.webhook)
		delete(response, "secret")
		return apiResponse(request, http.StatusOK, map[string]any{"data": response, "message": "Webhook updated successfully."})
	case request.Method == http.MethodDelete && path == "/v1/webhooks/"+testWebhookID:
		a.webhook = nil
		return apiResponse(request, http.StatusOK, map[string]any{"message": "Webhook deleted successfully."})
	default:
		return apiResponse(request, http.StatusNotFound, map[string]any{"message": fmt.Sprintf("No acceptance fixture for %s %s", request.Method, path)})
	}
}

func (a *acceptanceAPI) expectBody(actual, expected map[string]any) {
	a.t.Helper()
	if !reflect.DeepEqual(actual, expected) {
		a.t.Errorf("request body = %#v, want %#v", actual, expected)
	}
}

func (a *acceptanceAPI) read(request *http.Request, value map[string]any) (*http.Response, error) {
	if value == nil {
		return apiResponse(request, http.StatusNotFound, map[string]any{"message": "Not found."})
	}
	return apiResponse(request, http.StatusOK, value)
}

func (a *acceptanceAPI) setProjectName(name string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.project != nil {
		a.project["name"] = name
	}
}

func (a *acceptanceAPI) actionCount(name string) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.verificationInvokes[name]
}

func (a *acceptanceAPI) empty() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.project == nil && a.domain == nil && a.route == nil && a.webhook == nil
}

func apiResponse(request *http.Request, status int, value any) (*http.Response, error) {
	contents, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(contents)),
		Request:    request,
	}, nil
}

func requestJSON(request *http.Request) map[string]any {
	if request.Body == nil {
		return map[string]any{}
	}
	defer request.Body.Close()
	var value map[string]any
	if err := json.NewDecoder(request.Body).Decode(&value); err != nil {
		panic(err)
	}
	return value
}

func cloneMap(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func acceptanceProviderFactories(api *acceptanceAPI) map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"lettermint": providerserver.NewProtocol6WithError(&lettermintProvider{
			version: "test",
			clientFactory: func(token string) (*lettermint.APIClient, error) {
				return lettermint.NewAPI(
					token,
					lettermint.WithBaseURL("https://api.example.test/v1"),
					lettermint.WithHTTPClient(&http.Client{Transport: api}),
				)
			},
		}),
	}
}

func TestAccCurrentSDKSubset(t *testing.T) {
	if os.Getenv("TF_ACC") != "1" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	api := newAcceptanceAPI(t)
	baseConfig := acceptanceConfig(false)
	actionConfig := acceptanceConfig(true)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acceptanceProviderFactories(api),
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_14_0),
		},
		CheckDestroy: func(*terraform.State) error {
			if !api.empty() {
				return fmt.Errorf("acceptance resources remain")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: baseConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("lettermint_project.test", "id", testProjectID),
					resource.TestCheckResourceAttr("lettermint_project.test", "api_token", "lm_project_test"),
					resource.TestCheckResourceAttr("lettermint_domain.test", "dns_records.0.id", testRecordID),
					resource.TestCheckResourceAttr("lettermint_route.test", "inbound_domain", "support.example.com"),
					resource.TestCheckResourceAttr("lettermint_webhook.test", "secret", "whsec_test"),
				),
			},
			{
				Config: actionConfig,
				PostApplyFunc: func() {
					for _, name := range []string{"domain", "record", "route"} {
						if api.actionCount(name) != 1 {
							t.Errorf("%s verification count = %d, want 1", name, api.actionCount(name))
						}
					}
				},
			},
			{
				PreConfig:          func() { api.setProjectName("Remote drift") },
				Config:             actionConfig,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config: actionConfig,
			},
			{
				Config: baseConfig,
			},
			{
				ResourceName:            "lettermint_project.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"api_token", "initial_routes", "short_token"},
			},
			{
				ResourceName:            "lettermint_domain.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"dns_records"},
			},
			{
				ResourceName:      "lettermint_route.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:            "lettermint_webhook.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"secret"},
			},
		},
	})
}

func acceptanceConfig(withActions bool) string {
	actions := ""
	if withActions {
		actions = `
action "lettermint_verify_domain_dns" "test" {
  config {
    domain_id = lettermint_domain.test.id
  }
}

action "lettermint_verify_domain_dns_record" "test" {
  config {
    domain_id = lettermint_domain.test.id
    record_id = lettermint_domain.test.dns_records[0].id
  }
}

action "lettermint_verify_route_inbound_domain" "test" {
  config {
    route_id = lettermint_route.test.id
  }
}

resource "terraform_data" "dns" {
  input = {
    for record in lettermint_domain.test.dns_records : record.id => {
      type    = record.type
      fqdn    = record.fqdn
      content = record.content
    }
  }

  lifecycle {
    action_trigger {
      events = [after_create]
      actions = [
        action.lettermint_verify_domain_dns.test,
        action.lettermint_verify_domain_dns_record.test,
        action.lettermint_verify_route_inbound_domain.test,
      ]
    }
  }
}
`
	}
	return strings.TrimSpace(fmt.Sprintf(`
provider "lettermint" {
  team_token = "test-token"
}

resource "lettermint_project" "test" {
  name                 = "Acceptance project"
  smtp_enabled         = true
  initial_routes       = "both"
  short_token          = false
  redact_email_content = true
}

resource "lettermint_domain" "test" {
  domain = "example.com"
}

resource "lettermint_route" "test" {
  project_id             = lettermint_project.test.id
  name                   = "Support"
  route_type             = "inbound"
  inbound_domain         = "support.example.com"
  inbound_spam_threshold = 5
  attachment_delivery    = "url"
}

resource "lettermint_webhook" "test" {
  route_id = lettermint_route.test.id
  name     = "Delivery events"
  url      = "https://example.com/webhooks/lettermint"
  events   = ["message.delivered"]
  enabled  = true
}

%s
`, actions))
}
