package provider

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lettermint/lettermint-go/v2"
)

func stringValue(value string) types.String {
	return types.StringValue(value)
}

func nullableString(value *string) types.String {
	if value == nil {
		return types.StringNull()
	}
	return types.StringValue(*value)
}

func stringPointer(value types.String) *string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	v := value.ValueString()
	return &v
}

func boolPointer(value types.Bool) *bool {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	v := value.ValueBool()
	return &v
}

func floatPointer(value types.Float64) *float64 {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	v := value.ValueFloat64()
	return &v
}

func queryBool(value types.Bool) string {
	return strconv.FormatBool(value.ValueBool())
}

func queryList(ctx context.Context, value types.List, diags *diag.Diagnostics) (string, bool) {
	if value.IsNull() || value.IsUnknown() {
		return "", false
	}
	var items []string
	diags.Append(value.ElementsAs(ctx, &items, false)...)
	return strings.Join(items, ","), true
}

func isNotFound(err error) bool {
	var apiErr *lettermint.APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == 404
}

func requireClient(data any, target **clientData, diags *diag.Diagnostics) {
	if data == nil {
		return
	}
	client, ok := data.(*clientData)
	if !ok {
		diags.AddError("Unexpected provider data", fmt.Sprintf("Expected *clientData, got %T.", data))
		return
	}
	*target = client
}
