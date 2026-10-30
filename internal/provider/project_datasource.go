package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource                   = &ProjectDataSource{}
	_ datasource.DataSourceWithConfigure      = &ProjectDataSource{}
	_ datasource.DataSourceWithValidateConfig = &ProjectDataSource{}
)

const getProjectPath = "/provider/projects/get"

func NewProjectDataSource() datasource.DataSource {
	return &ProjectDataSource{}
}

type ProjectDataSource struct {
	providerData *FlowsProviderConfiguredData
}

// ProjectModel is the Terraform representation of a project. It is used both as
// the state of the flows_project data source and as the element type of the
// flows_projects data source.
type ProjectModel struct {
	ID                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	IsPersonalSandbox types.Bool   `tfsdk:"is_personal_sandbox"`
}

// Project is a project as returned by the Flows provider API.
type Project struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	IsPersonalSandbox bool   `json:"isPersonalSandbox"`
}

func (p Project) toModel() ProjectModel {
	return ProjectModel{
		ID:                types.StringValue(p.ID),
		Name:              types.StringValue(p.Name),
		IsPersonalSandbox: types.BoolValue(p.IsPersonalSandbox),
	}
}

type GetProjectRequest struct {
	// Exactly one of ProjectID or Name must be set.
	ProjectID *string `json:"projectId,omitempty"`
	Name      *string `json:"name,omitempty"`
}

type GetProjectResponse struct {
	Project Project `json:"project"`
}

func (ds *ProjectDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (ds *ProjectDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Data source for looking up a single project by name or ID, so that project IDs don't have to be hard-coded in configuration. " +
			"Exactly one of `id` or `name` must be set. The lookup fails if no project matches. " +
			"Project names are unique within an organization (ignoring case), so a name always resolves to at most one project. " +
			"Only projects the authentication token has at least viewer access to can be looked up.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "ID of the project. Set it to look the project up by ID, otherwise it is computed from the lookup by name.",
				Optional:    true,
				Computed:    true,
			},
			"name": schema.StringAttribute{
				Description: "Name of the project. Set it to look the project up by name (case-insensitive), otherwise it is computed from the lookup by ID.",
				Optional:    true,
				Computed:    true,
			},
			"is_personal_sandbox": schema.BoolAttribute{
				Description: "Whether the project is a user's automatically created personal sandbox.",
				Computed:    true,
			},
		},
	}
}

func (ds *ProjectDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	ds.providerData = req.ProviderData.(*FlowsProviderConfiguredData)
}

func (ds *ProjectDataSource) ValidateConfig(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	var data ProjectModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Unknown values are only resolved at apply time, so the check is left to Read.
	if data.ID.IsUnknown() || data.Name.IsUnknown() {
		return
	}

	if data.ID.IsNull() == data.Name.IsNull() {
		resp.Diagnostics.AddError("Invalid project selector", "Exactly one of `id` or `name` must be set to look up a project.")
	}
}

func (ds *ProjectDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ProjectModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var apiReq GetProjectRequest
	switch {
	case !data.ID.IsNull() && data.Name.IsNull():
		id := data.ID.ValueString()
		apiReq.ProjectID = &id
	case data.ID.IsNull() && !data.Name.IsNull():
		name := data.Name.ValueString()
		apiReq.Name = &name
	default:
		resp.Diagnostics.AddError("Invalid project selector", "Exactly one of `id` or `name` must be set to look up a project.")
		return
	}

	projectResp, err := CallFlowsAPI[GetProjectRequest, GetProjectResponse](*ds.providerData, getProjectPath, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", "Unable to read project, got error: "+err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, projectResp.Project.toModel())...)
}
