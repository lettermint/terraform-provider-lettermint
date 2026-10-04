package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lettermint/lettermint-go/v3"
)

func TestRouteUpdateRequestSendsOnlyKnownValues(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		model routeResourceModel
		want  map[string]any
	}{
		{
			name:  "nothing known",
			model: routeResourceModel{TrackOpens: types.BoolUnknown(), TLS: types.StringNull(), InboundDomain: types.StringUnknown(), InboundSpamThreshold: types.Float64Null()},
			want:  map[string]any{},
		},
		{
			name: "known settings and inbound settings",
			model: routeResourceModel{
				TrackOpens: types.BoolValue(false), TrackClicks: types.BoolUnknown(), TLS: types.StringValue("enforced"),
				InboundDomain: types.StringValue("support.example.com"), InboundSpamThreshold: types.Float64Value(0), AttachmentDelivery: types.StringNull(),
			},
			want: map[string]any{
				"settings":         map[string]any{"track_opens": false, "tls": "enforced"},
				"inbound_settings": map[string]any{"inbound_domain": "support.example.com", "inbound_spam_threshold": float64(0)},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			encoded, err := json.Marshal(routeUpdateRequest(test.model))
			if err != nil {
				t.Fatal(err)
			}
			var got map[string]any
			if err := json.Unmarshal(encoded, &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("request = %s, want %#v", encoded, test.want)
			}
		})
	}
}

func TestProjectUpdateRequestLeavesUnknownValuesAbsent(t *testing.T) {
	t.Parallel()

	encoded, err := json.Marshal(lettermint.UpdateProjectData{
		Name:               knownString(types.StringValue("Production")),
		SMTPEnabled:        knownBool(types.BoolValue(false)),
		RedactEmailContent: knownBool(types.BoolUnknown()),
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `{"name":"Production","smtp_enabled":false}` {
		t.Fatalf("request = %s", encoded)
	}
}

func TestRouteModelReadsOptionalAndNullableFields(t *testing.T) {
	t.Parallel()

	var route lettermint.RouteData
	if err := json.Unmarshal([]byte(`{"id":"route-1","inbound_address":null,"inbound_domain":"support.example.com","inbound_spam_threshold":0,"attachment_delivery":"url","settings":{"track_opens":false,"tls":"opportunistic"}}`), &route); err != nil {
		t.Fatal(err)
	}
	model := routeModelFromAPI(route)
	if !model.InboundAddress.IsNull() || model.InboundDomain.ValueString() != "support.example.com" || !model.InboundDomainVerifiedAt.IsNull() {
		t.Fatalf("inbound strings = %#v", model)
	}
	if model.InboundSpamThreshold.IsNull() || model.InboundSpamThreshold.ValueFloat64() != 0 || model.AttachmentDelivery.ValueString() != "url" {
		t.Fatalf("inbound settings = %#v", model)
	}
	if model.TrackOpens.IsNull() || model.TrackOpens.ValueBool() || !model.TrackClicks.IsNull() || model.TLS.ValueString() != "opportunistic" {
		t.Fatalf("settings = %#v", model)
	}

	var withoutSettings lettermint.RouteData
	if err := json.Unmarshal([]byte(`{"id":"route-1","settings":null}`), &withoutSettings); err != nil {
		t.Fatal(err)
	}
	model = routeModelFromAPI(withoutSettings)
	if !model.TrackOpens.IsNull() || !model.TLS.IsNull() || !model.AttachmentDelivery.IsNull() || !model.InboundSpamThreshold.IsNull() {
		t.Fatalf("absent settings = %#v", model)
	}
}

func TestIsNotFoundUsesTheSDKErrorType(t *testing.T) {
	t.Parallel()

	for status, want := range map[int]bool{http.StatusNotFound: true, http.StatusForbidden: false, http.StatusInternalServerError: false} {
		client := dataSourceClient(t, func(request *http.Request) (*http.Response, error) {
			return apiResponse(request, status, map[string]any{"message": http.StatusText(status)})
		})
		_, err := client.Routes.Retrieve(context.Background(), testRouteID, nil)
		if err == nil || isNotFound(err) != want {
			t.Fatalf("status %d: isNotFound(%v) = %t, want %t", status, err, isNotFound(err), want)
		}
	}
}
