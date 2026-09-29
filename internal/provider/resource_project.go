package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lettermint/lettermint-go/v2"
)

var (
	_ resource.Resource                = (*projectResource)(nil)
	_ resource.ResourceWithConfigure   = (*projectResource)(nil)
	_ resource.ResourceWithImportState = (*projectResource)(nil)
)

type projectResource struct {
	client *clientData
}

type projectResourceModel struct {
	ID                 types.String `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	SMTPEnabled        types.Bool   `tfsdk:"smtp_enabled"`
	InitialRoutes      types.String `tfsdk:"initial_routes"`
	ShortToken         types.Bool   `tfsdk:"short_token"`
	RedactEmailContent types.Bool   `tfsdk:"redact_email_content"`
	DefaultRouteID     types.String `tfsdk:"default_route_id"`
	APIToken           types.String `tfsdk:"api_token"`
	TokenGeneratedAt   types.String `tfsdk:"token_generated_at"`
	TokenLastUsedAt    types.String `tfsdk:"token_last_used_at"`
	TokenLastUsedIP    types.String `tfsdk:"token_last_used_ip"`
	CreatedAt          types.String `tfsdk:"created_at"`
	UpdatedAt          types.String `tfsdk:"updated_at"`
}

func NewProjectResource() resource.Resource {
	return &projectResource{}
}

func (r *projectResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (r *projectResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage a Lettermint project.",
		Attributes: map[string]schema.Attribute{
			"id": computedIDAttribute(),
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Project name.",
				Validators:  []validator.String{stringvalidator.LengthAtMost(255)},
			},
			"smtp_enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether SMTP sending is enabled.",
			},
			"initial_routes": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Default:       stringdefault.StaticString("both"),
				Description:   "Routes created with the project.",
				Validators:    enum("both", "transactional", "broadcast"),
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"short_token": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Default:       booldefault.StaticBool(false),
				Description:   "Whether the create operation returns a short API token.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
			},
			"redact_email_content": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether Lettermint redacts stored email content.",
			},
			"default_route_id": schema.StringAttribute{
				Computed:    true,
				Description: "Default route ID.",
			},
			"api_token": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "API token returned once when the project is created.",
			},
			"token_generated_at": schema.StringAttribute{Computed: true, Description: "Token generation time."},
			"token_last_used_at": schema.StringAttribute{Computed: true, Description: "Last token use time."},
			"token_last_used_ip": schema.StringAttribute{Computed: true, Description: "Last token use IP address."},
			"created_at":         schema.StringAttribute{Computed: true, Description: "Creation time."},
			"updated_at":         schema.StringAttribute{Computed: true, Description: "Last update time."},
		},
	}
}

func (r *projectResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	requireClient(req.ProviderData, &r.client, &resp.Diagnostics)
}

func (r *projectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan projectResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.API.Projects.Create(ctx, lettermint.ProjectStoreRequest{
		Name:          plan.Name.ValueString(),
		SMTPEnabled:   boolPointer(plan.SMTPEnabled),
		InitialRoutes: lettermint.InitialRoutes(plan.InitialRoutes.ValueString()),
		ShortToken:    boolPointer(plan.ShortToken),
	})
	if err != nil {
		appendClientDiagnostic(&resp.Diagnostics, "Cannot create project", err)
		return
	}
	project := result.Data
	if !plan.RedactEmailContent.IsNull() && !plan.RedactEmailContent.IsUnknown() && project.RedactEmailContent != plan.RedactEmailContent.ValueBool() {
		updated, updateErr := r.client.API.Projects.Update(ctx, project.ID, lettermint.ProjectUpdateRequest{
			RedactEmailContent: boolPointer(plan.RedactEmailContent),
		})
		if updateErr != nil {
			appendClientDiagnostic(&resp.Diagnostics, "Project was created, but its settings could not be updated", updateErr)
			return
		}
		project = updated.Data
	}

	state := projectModelFromAPI(project, plan)
	state.APIToken = types.StringValue(result.APIToken)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *projectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state projectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.API.Projects.Retrieve(ctx, state.ID.ValueString())
	if isNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		appendClientDiagnostic(&resp.Diagnostics, "Cannot read project", err)
		return
	}

	state = projectModelFromAPI(lettermint.ProjectData(result), state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *projectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan projectResourceModel
	var state projectResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.API.Projects.Update(ctx, state.ID.ValueString(), lettermint.ProjectUpdateRequest{
		Name:               stringPointer(plan.Name),
		SMTPEnabled:        boolPointer(plan.SMTPEnabled),
		RedactEmailContent: boolPointer(plan.RedactEmailContent),
	})
	if err != nil {
		appendClientDiagnostic(&resp.Diagnostics, "Cannot update project", err)
		return
	}

	plan.ID = state.ID
	plan.APIToken = state.APIToken
	plan = projectModelFromAPI(result.Data, plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *projectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state projectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, err := r.client.API.Projects.Delete(ctx, state.ID.ValueString())
	if err != nil && !isNotFound(err) {
		appendClientDiagnostic(&resp.Diagnostics, "Cannot delete project", err)
	}
}

func (r *projectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func projectModelFromAPI(project lettermint.ProjectData, previous projectResourceModel) projectResourceModel {
	previous.ID = stringValue(project.ID)
	previous.Name = stringValue(project.Name)
	previous.SMTPEnabled = types.BoolValue(project.SMTPEnabled)
	previous.RedactEmailContent = types.BoolValue(project.RedactEmailContent)
	previous.DefaultRouteID = nullableString(project.DefaultRouteID)
	previous.TokenGeneratedAt = nullableString(project.TokenGeneratedAt)
	previous.TokenLastUsedAt = nullableString(project.TokenLastUsedAt)
	previous.TokenLastUsedIP = nullableString(project.TokenLastUsedIp)
	previous.CreatedAt = stringValue(project.CreatedAt)
	previous.UpdatedAt = stringValue(project.UpdatedAt)
	return previous
}
