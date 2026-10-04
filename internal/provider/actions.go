package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/action"
	actionschema "github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ action.ActionWithConfigure = (*verifyDomainDNSAction)(nil)
	_ action.ActionWithConfigure = (*verifyDomainDNSRecordAction)(nil)
	_ action.ActionWithConfigure = (*verifyRouteInboundDomainAction)(nil)
)

type verifyDomainDNSAction struct{ client *clientData }
type verifyDomainDNSRecordAction struct{ client *clientData }
type verifyRouteInboundDomainAction struct{ client *clientData }

type verifyDomainDNSActionModel struct {
	DomainID types.String `tfsdk:"domain_id"`
}
type verifyDomainDNSRecordActionModel struct {
	DomainID types.String `tfsdk:"domain_id"`
	RecordID types.String `tfsdk:"record_id"`
}
type verifyRouteInboundDomainActionModel struct {
	RouteID types.String `tfsdk:"route_id"`
}

func NewVerifyDomainDNSAction() action.Action          { return &verifyDomainDNSAction{} }
func NewVerifyDomainDNSRecordAction() action.Action    { return &verifyDomainDNSRecordAction{} }
func NewVerifyRouteInboundDomainAction() action.Action { return &verifyRouteInboundDomainAction{} }

func configureAction(data any, target **clientData, resp *action.ConfigureResponse) {
	requireClient(data, target, &resp.Diagnostics)
}

func (a *verifyDomainDNSAction) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_verify_domain_dns"
}
func (a *verifyDomainDNSAction) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = actionschema.Schema{Description: "Verify all DNS records for a Lettermint domain.", Attributes: map[string]actionschema.Attribute{
		"domain_id": actionschema.StringAttribute{Required: true, Description: "Domain ID."},
	}}
}
func (a *verifyDomainDNSAction) Configure(_ context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
	configureAction(req.ProviderData, &a.client, resp)
}
func (a *verifyDomainDNSAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var config verifyDomainDNSActionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := a.client.Domains.VerifyDNSRecords(ctx, config.DomainID.ValueString())
	if err != nil {
		appendClientDiagnostic(&resp.Diagnostics, "Cannot verify domain DNS records", err)
		return
	}
	if resp.SendProgress != nil {
		resp.SendProgress(action.InvokeProgressEvent{Message: result.Message})
	}
	if len(result.RecommendedFailedRecords) > 0 {
		resp.Diagnostics.AddWarning("Recommended DNS records did not verify", fmt.Sprintf("Lettermint reported %d recommended DNS record(s) that did not verify.", len(result.RecommendedFailedRecords)))
	}
}

func (a *verifyDomainDNSRecordAction) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_verify_domain_dns_record"
}
func (a *verifyDomainDNSRecordAction) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = actionschema.Schema{Description: "Verify one DNS record for a Lettermint domain.", Attributes: map[string]actionschema.Attribute{
		"domain_id": actionschema.StringAttribute{Required: true, Description: "Domain ID."},
		"record_id": actionschema.StringAttribute{Required: true, Description: "DNS record ID."},
	}}
}
func (a *verifyDomainDNSRecordAction) Configure(_ context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
	configureAction(req.ProviderData, &a.client, resp)
}
func (a *verifyDomainDNSRecordAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var config verifyDomainDNSRecordActionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := a.client.Domains.VerifyDNSRecord(ctx, config.DomainID.ValueString(), config.RecordID.ValueString())
	if err != nil {
		appendClientDiagnostic(&resp.Diagnostics, "Cannot verify domain DNS record", err)
		return
	}
	if resp.SendProgress != nil {
		resp.SendProgress(action.InvokeProgressEvent{Message: result.Message})
	}
}

func (a *verifyRouteInboundDomainAction) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_verify_route_inbound_domain"
}
func (a *verifyRouteInboundDomainAction) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = actionschema.Schema{Description: "Verify the inbound domain for a Lettermint route.", Attributes: map[string]actionschema.Attribute{
		"route_id": actionschema.StringAttribute{Required: true, Description: "Route ID."},
	}}
}
func (a *verifyRouteInboundDomainAction) Configure(_ context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
	configureAction(req.ProviderData, &a.client, resp)
}
func (a *verifyRouteInboundDomainAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var config verifyRouteInboundDomainActionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := a.client.Routes.VerifyInboundDomain(ctx, config.RouteID.ValueString())
	if err != nil {
		appendClientDiagnostic(&resp.Diagnostics, "Cannot verify route inbound domain", err)
		return
	}
	if resp.SendProgress != nil {
		resp.SendProgress(action.InvokeProgressEvent{Message: result.Data.Message})
	}
}
