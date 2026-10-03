package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lettermint/lettermint-go/v3"
)

var (
	_ resource.Resource                = (*domainResource)(nil)
	_ resource.ResourceWithConfigure   = (*domainResource)(nil)
	_ resource.ResourceWithImportState = (*domainResource)(nil)
)

type domainResource struct {
	client *clientData
}

type domainResourceModel struct {
	ID              types.String `tfsdk:"id"`
	Domain          types.String `tfsdk:"domain"`
	DKIMMode        types.String `tfsdk:"dkim_mode"`
	RotationReady   types.Bool   `tfsdk:"rotation_ready"`
	StatusChangedAt types.String `tfsdk:"status_changed_at"`
	DNSRecords      types.List   `tfsdk:"dns_records"`
	CreatedAt       types.String `tfsdk:"created_at"`
}

type domainDNSRecordModel struct {
	ID                      types.String `tfsdk:"id"`
	Type                    types.String `tfsdk:"type"`
	Hostname                types.String `tfsdk:"hostname"`
	FQDN                    types.String `tfsdk:"fqdn"`
	Content                 types.String `tfsdk:"content"`
	Status                  types.String `tfsdk:"status"`
	Purpose                 types.String `tfsdk:"purpose"`
	VerificationScope       types.String `tfsdk:"verification_scope"`
	RequiredForVerification types.Bool   `tfsdk:"required_for_verification"`
	VerifiedAt              types.String `tfsdk:"verified_at"`
	LastCheckedAt           types.String `tfsdk:"last_checked_at"`
}

func NewDomainResource() resource.Resource {
	return &domainResource{}
}

func (r *domainResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain"
}

func (r *domainResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage a Lettermint sending domain.",
		Attributes: map[string]schema.Attribute{
			"id": computedIDAttribute(),
			"domain": schema.StringAttribute{
				Required:      true,
				Description:   "Domain name.",
				Validators:    domainNameValidators(),
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"dkim_mode":         schema.StringAttribute{Computed: true, Description: "DKIM management mode."},
			"rotation_ready":    schema.BoolAttribute{Computed: true, Description: "Whether DKIM rotation is ready."},
			"status_changed_at": schema.StringAttribute{Computed: true, Description: "Last domain status change time."},
			"dns_records": schema.ListNestedAttribute{
				Computed:    true,
				Description: "DNS records returned when the domain is created. A later read preserves these records when the API omits the optional relationship.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id":                        schema.StringAttribute{Computed: true, Description: "DNS record ID."},
					"type":                      schema.StringAttribute{Computed: true, Description: "DNS record type."},
					"hostname":                  schema.StringAttribute{Computed: true, Description: "DNS record hostname."},
					"fqdn":                      schema.StringAttribute{Computed: true, Description: "Fully qualified DNS name."},
					"content":                   schema.StringAttribute{Computed: true, Description: "DNS record content."},
					"status":                    schema.StringAttribute{Computed: true, Description: "DNS verification status."},
					"purpose":                   schema.StringAttribute{Computed: true, Description: "DNS record purpose."},
					"verification_scope":        schema.StringAttribute{Computed: true, Description: "DNS verification scope."},
					"required_for_verification": schema.BoolAttribute{Computed: true, Description: "Whether Lettermint requires this record for verification."},
					"verified_at":               schema.StringAttribute{Computed: true, Description: "Verification time."},
					"last_checked_at":           schema.StringAttribute{Computed: true, Description: "Last verification check time."},
				}},
			},
			"created_at": schema.StringAttribute{Computed: true, Description: "Creation time."},
		},
	}
}

func (r *domainResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	requireClient(req.ProviderData, &r.client, &resp.Diagnostics)
}

func (r *domainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan domainResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.Domains.Create(ctx, lettermint.StoreDomainData{Domain: plan.Domain.ValueString()})
	if err != nil {
		appendClientDiagnostic(&resp.Diagnostics, "Cannot create domain", err)
		return
	}

	state := domainModelFromAPI(*result, plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *domainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state domainResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.Domains.Retrieve(ctx, state.ID.ValueString(), nil)
	if isNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		appendClientDiagnostic(&resp.Diagnostics, "Cannot read domain", err)
		return
	}

	state = domainModelFromAPI(*result, state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *domainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan domainResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
}

func (r *domainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state domainResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, err := r.client.Domains.Delete(ctx, state.ID.ValueString())
	if err != nil && !isNotFound(err) {
		appendClientDiagnostic(&resp.Diagnostics, "Cannot delete domain", err)
	}
}

func (r *domainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func domainModelFromAPI(domain lettermint.DomainData, previous domainResourceModel) domainResourceModel {
	previous.ID = stringValue(domain.ID)
	previous.Domain = stringValue(domain.Domain)
	previous.DKIMMode = stringValue(string(domain.DkimMode))
	previous.RotationReady = types.BoolValue(domain.RotationReady)
	previous.StatusChangedAt = nullableString(domain.StatusChangedAt)
	previous.CreatedAt = stringValue(domain.CreatedAt)
	if domain.DNSRecords != nil {
		elements := make([]attr.Value, 0, len(domain.DNSRecords))
		for _, record := range domain.DNSRecords {
			elements = append(elements, domainDNSRecordValue(record))
		}
		previous.DNSRecords = types.ListValueMust(domainDNSRecordObjectType(), elements)
	} else if previous.DNSRecords.IsUnknown() {
		previous.DNSRecords = types.ListNull(domainDNSRecordObjectType())
	}
	return previous
}

func domainDNSRecordObjectType() types.ObjectType {
	return types.ObjectType{AttrTypes: map[string]attr.Type{
		"id":                        types.StringType,
		"type":                      types.StringType,
		"hostname":                  types.StringType,
		"fqdn":                      types.StringType,
		"content":                   types.StringType,
		"status":                    types.StringType,
		"purpose":                   types.StringType,
		"verification_scope":        types.StringType,
		"required_for_verification": types.BoolType,
		"verified_at":               types.StringType,
		"last_checked_at":           types.StringType,
	}}
}

func domainDNSRecordValue(record lettermint.DomainDnsRecordData) types.Object {
	return types.ObjectValueMust(domainDNSRecordObjectType().AttrTypes, map[string]attr.Value{
		"id":                        stringValue(record.ID),
		"type":                      stringValue(string(record.Type)),
		"hostname":                  stringValue(record.Hostname),
		"fqdn":                      stringValue(record.Fqdn),
		"content":                   stringValue(record.Content),
		"status":                    stringValue(string(record.Status)),
		"purpose":                   stringValue(string(record.Purpose)),
		"verification_scope":        stringValue(string(record.VerificationScope)),
		"required_for_verification": types.BoolValue(record.RequiredForVerification),
		"verified_at":               nullableString(record.VerifiedAt),
		"last_checked_at":           nullableString(record.LastCheckedAt),
	})
}
