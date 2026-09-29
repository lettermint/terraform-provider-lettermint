package provider

import (
	"context"
	"net/url"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var domainNamePattern = regexp.MustCompile(`^((.{1,63}\.)+[a-zA-Z]{2,63})$`)
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type domainNameValidator struct{}
type absoluteURIValidator struct{}
type uuidValidator struct{}

func (domainNameValidator) Description(context.Context) string {
	return "value must match the domain format in the Lettermint OpenAPI document"
}

func (v domainNameValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v domainNameValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueString()
	if len(value) > 255 || strings.HasPrefix(value, "://") || !domainNamePattern.MatchString(value) {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid domain name",
			"The domain name must match the validation rule in the Lettermint OpenAPI document.",
		)
	}
}

func (absoluteURIValidator) Description(context.Context) string {
	return "value must be an absolute URI"
}

func (v absoluteURIValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v absoluteURIValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	parsed, err := url.ParseRequestURI(req.ConfigValue.ValueString())
	if err != nil || !parsed.IsAbs() {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid URI", "The value must use the URI format in the Lettermint OpenAPI document.")
	}
}

func (uuidValidator) Description(context.Context) string {
	return "value must use UUID format"
}

func (v uuidValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v uuidValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	if !uuidPattern.MatchString(req.ConfigValue.ValueString()) {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid UUID", "The value must use the UUID format in the Lettermint OpenAPI document.")
	}
}

func computedIDAttribute() schema.StringAttribute {
	return schema.StringAttribute{
		Computed:    true,
		Description: "Lettermint resource ID.",
	}
}

func requiresReplace() []planmodifier.String {
	return []planmodifier.String{stringplanmodifier.RequiresReplace()}
}

func enum(values ...string) []validator.String {
	return []validator.String{stringvalidator.OneOf(values...)}
}

func domainNameValidators() []validator.String {
	return []validator.String{domainNameValidator{}}
}

func absoluteURIValidators() []validator.String {
	return []validator.String{absoluteURIValidator{}}
}

func uuidValidators() []validator.String {
	return []validator.String{uuidValidator{}}
}
