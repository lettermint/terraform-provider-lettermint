package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func validateString(value string, underTest validator.String) bool {
	request := validator.StringRequest{Path: path.Root("value"), ConfigValue: types.StringValue(value)}
	var response validator.StringResponse
	underTest.ValidateString(context.Background(), request, &response)
	return response.Diagnostics.HasError()
}

func TestOpenAPIStringValidators(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		value     string
		validator validator.String
		wantError bool
	}{
		{name: "valid domain", value: "mail.example.com", validator: domainNameValidator{}},
		{name: "domain needs a dot", value: "example", validator: domainNameValidator{}, wantError: true},
		{name: "UUID syntax permits zero values", value: "00000000-0000-0000-0000-000000000000", validator: uuidValidator{}},
		{name: "invalid UUID", value: "not-a-uuid", validator: uuidValidator{}, wantError: true},
		{name: "absolute URI", value: "mailto:ops@example.com", validator: absoluteURIValidator{}},
		{name: "relative URI", value: "/webhooks", validator: absoluteURIValidator{}, wantError: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if got := validateString(testCase.value, testCase.validator); got != testCase.wantError {
				t.Fatalf("validation error = %t, want %t", got, testCase.wantError)
			}
		})
	}
}
