package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
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
	ProjectIDs      types.Set    `tfsdk:"project_ids"`
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
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Lettermint resource ID.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
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
				Computed:      true,
				Description:   "DNS records returned when the domain is created. A later read preserves these records when the API omits the optional relationship.",
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
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
			"project_ids": schema.SetAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "IDs of the projects that may send from the domain. An empty set lets every project in the team send from it. When this argument is not set, the provider does not read or change the project assignments.",
				Validators:  []validator.Set{setvalidator.ValueStringsAre(uuidValidators()...)},
			},
			"created_at": schema.StringAttribute{
				Computed:      true,
				Description:   "Creation time.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
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
	projectIDs := projectIDsFromPlan(ctx, plan.ProjectIDs, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(projectIDs) == 0 {
		// A new domain has no projects, so it is already available to every project.
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	// Save the domain before the assignment, so that Terraform keeps track of it if the assignment fails.
	state.ProjectIDs = domainProjectIDsValue(nil)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := r.client.Domains.UpdateProjects(ctx, state.ID.ValueString(), lettermint.UpdateDomainProjectsData{ProjectIDs: projectIDs})
	if err != nil {
		appendClientDiagnostic(&resp.Diagnostics, "Cannot assign projects to domain", err)
		return
	}

	state = domainModelFromAPI(updated.Data, state)
	state.ProjectIDs = domainProjectIDsValue(updated.Data.Projects)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *domainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state domainResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Ask for projects only when the configuration manages them, because the include needs project read access.
	var query *lettermint.GetDomainQuery
	if !state.ProjectIDs.IsNull() {
		query = &lettermint.GetDomainQuery{Include: []lettermint.GetDomainQueryIncludeItem{lettermint.GetDomainQueryIncludeItemProjects}}
	}

	result, err := r.client.Domains.Retrieve(ctx, state.ID.ValueString(), query)
	if isNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		appendClientDiagnostic(&resp.Diagnostics, "Cannot read domain", err)
		return
	}

	state = domainModelFromAPI(*result, state)
	if query != nil {
		state.ProjectIDs = domainProjectIDsValue(result.Projects)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *domainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan domainResourceModel
	var state domainResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// project_ids is the only argument that changes in place. A removed argument stops
	// the management of the projects and leaves the assignments in Lettermint unchanged.
	if plan.ProjectIDs.IsNull() || plan.ProjectIDs.Equal(state.ProjectIDs) {
		state.ProjectIDs = plan.ProjectIDs
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	projectIDs := projectIDsFromPlan(ctx, plan.ProjectIDs, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.Domains.UpdateProjects(ctx, state.ID.ValueString(), lettermint.UpdateDomainProjectsData{ProjectIDs: projectIDs})
	if err != nil {
		appendClientDiagnostic(&resp.Diagnostics, "Cannot update domain projects", err)
		return
	}

	dnsRecords := state.DNSRecords
	state = domainModelFromAPI(result.Data, state)
	state.DNSRecords = dnsRecords
	state.ProjectIDs = domainProjectIDsValue(result.Data.Projects)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
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

// projectIDsFromPlan returns the planned project IDs. The result is never nil,
// because the API reads a JSON null differently from an empty list.
func projectIDsFromPlan(ctx context.Context, value types.Set, diags *diag.Diagnostics) []string {
	projectIDs := []string{}
	if value.IsNull() || value.IsUnknown() {
		return projectIDs
	}
	diags.Append(value.ElementsAs(ctx, &projectIDs, false)...)
	if projectIDs == nil {
		projectIDs = []string{}
	}
	return projectIDs
}

// domainProjectIDsValue maps the projects of a domain to a set of IDs. An absent
// list is an empty set, because a domain without projects is available to every project.
func domainProjectIDsValue(projects []lettermint.DomainDataProjectsItem) types.Set {
	elements := make([]attr.Value, 0, len(projects))
	for _, project := range projects {
		elements = append(elements, types.StringValue(project.ID))
	}
	return types.SetValueMust(types.StringType, elements)
}
