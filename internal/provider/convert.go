package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lettermint/lettermint-go/v3"
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

// optionalString reads an optional and nullable API string. An absent or null
// value is a null Terraform value.
func optionalString(value lettermint.Nullable[string]) types.String {
	if v, ok := value.Get(); ok {
		return types.StringValue(v)
	}
	return types.StringNull()
}

// optionalFloat64 reads an optional and nullable API number. An absent or null
// value is a null Terraform value.
func optionalFloat64(value lettermint.Nullable[float64]) types.Float64 {
	if v, ok := value.Get(); ok {
		return types.Float64Value(v)
	}
	return types.Float64Null()
}

func optionalBool(value *bool) types.Bool {
	if value == nil {
		return types.BoolNull()
	}
	return types.BoolValue(*value)
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

// knownString maps a planned value to an optional and nullable request field.
// A known value is sent. A null or unknown value is absent, so the API keeps
// its current value. The provider never sends an explicit null here.
func knownString(value types.String) lettermint.Nullable[string] {
	if value.IsNull() || value.IsUnknown() {
		return lettermint.Nullable[string]{}
	}
	return lettermint.Value(value.ValueString())
}

// knownBool maps a planned value like knownString.
func knownBool(value types.Bool) lettermint.Nullable[bool] {
	if value.IsNull() || value.IsUnknown() {
		return lettermint.Nullable[bool]{}
	}
	return lettermint.Value(value.ValueBool())
}

// knownFloat64 maps a planned value like knownString.
func knownFloat64(value types.Float64) lettermint.Nullable[float64] {
	if value.IsNull() || value.IsUnknown() {
		return lettermint.Nullable[float64]{}
	}
	return lettermint.Value(value.ValueFloat64())
}

// knownEnum maps a planned string to an optional and nullable enum request
// field like knownString.
func knownEnum[T ~string](value types.String) lettermint.Nullable[T] {
	if value.IsNull() || value.IsUnknown() {
		return lettermint.Nullable[T]{}
	}
	return lettermint.Value(T(value.ValueString()))
}

func queryString(value types.String) string {
	if value.IsNull() || value.IsUnknown() {
		return ""
	}
	return value.ValueString()
}

func queryInt(value types.Int64) int {
	if value.IsNull() || value.IsUnknown() {
		return 0
	}
	return int(value.ValueInt64())
}

func queryList[T ~string](ctx context.Context, value types.List, diags *diag.Diagnostics) []T {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	var items []string
	diags.Append(value.ElementsAs(ctx, &items, false)...)
	result := make([]T, 0, len(items))
	for _, item := range items {
		result = append(result, T(item))
	}
	return result
}

func isNotFound(err error) bool {
	var notFound *lettermint.NotFoundError
	return errors.As(err, &notFound)
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
