package provider

import (
	"context"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource                   = &ProjectsDataSource{}
	_ datasource.DataSourceWithConfigure      = &ProjectsDataSource{}
	_ datasource.DataSourceWithValidateConfig = &ProjectsDataSource{}
)

const listProjectsPath = "/provider/projects/list"

func NewProjectsDataSource() datasource.DataSource {
	return &ProjectsDataSource{}
}

type ProjectsDataSource struct {
	providerData *FlowsProviderConfiguredData
}

type ProjectsDataSourceModel struct {
	NameRegex         types.String   `tfsdk:"name_regex"`
	IsPersonalSandbox types.Bool     `tfsdk:"is_personal_sandbox"`
	IDs               types.Set      `tfsdk:"ids"`
	Projects          []ProjectModel `tfsdk:"projects"`
}

type ListProjectsRequest struct{}

type ListProjectsResponse struct {
	Projects []Project `json:"projects"`
}

func (ds *ProjectsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_projects"
}

func (ds *ProjectsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Data source for listing the projects the authentication token has at least viewer access to, optionally filtered by name pattern and personal-sandbox status. " +
			"Projects are returned sorted by name. The `ids` attribute is a set of project IDs, so it can be used directly with `for_each`. " +
			"When no project matches the filters, the result is empty rather than an error.",
		Attributes: map[string]schema.Attribute{
			"name_regex": schema.StringAttribute{
				Description: "Only include projects whose name matches this regular expression (RE2 syntax, case-sensitive unless the pattern says otherwise, e.g. `(?i)sandbox`).",
				Optional:    true,
			},
			"is_personal_sandbox": schema.BoolAttribute{
				Description: "Only include projects that are (true) or are not (false) users' automatically created personal sandboxes.",
				Optional:    true,
			},
			"ids": schema.SetAttribute{
				Description: "Set of the IDs of the matching projects.",
				Computed:    true,
				ElementType: types.StringType,
			},
			"projects": schema.ListNestedAttribute{
				Description: "List of the matching projects, sorted by name.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Description: "ID of the project.",
							Computed:    true,
						},
						"name": schema.StringAttribute{
							Description: "Name of the project.",
							Computed:    true,
						},
						"is_personal_sandbox": schema.BoolAttribute{
							Description: "Whether the project is a user's automatically created personal sandbox.",
							Computed:    true,
						},
					},
				},
			},
		},
	}
}

func (ds *ProjectsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	ds.providerData = req.ProviderData.(*FlowsProviderConfiguredData)
}

func (ds *ProjectsDataSource) ValidateConfig(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	var data ProjectsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.NameRegex.IsNull() || data.NameRegex.IsUnknown() {
		return
	}

	if _, err := regexp.Compile(data.NameRegex.ValueString()); err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("name_regex"), "Invalid name_regex", "The name_regex attribute must be a valid regular expression: "+err.Error())
	}
}

func (ds *ProjectsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ProjectsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var nameRegex *regexp.Regexp
	if !data.NameRegex.IsNull() {
		var err error
		nameRegex, err = regexp.Compile(data.NameRegex.ValueString())
		if err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("name_regex"), "Invalid name_regex", "The name_regex attribute must be a valid regular expression: "+err.Error())
			return
		}
	}

	listResp, err := CallFlowsAPI[ListProjectsRequest, ListProjectsResponse](*ds.providerData, listProjectsPath, ListProjectsRequest{})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", "Unable to list projects, got error: "+err.Error())
		return
	}

	// The API returns every project visible to the token; filtering happens here.
	ids := make([]string, 0, len(listResp.Projects))
	projects := make([]ProjectModel, 0, len(listResp.Projects))
	for _, project := range listResp.Projects {
		if nameRegex != nil && !nameRegex.MatchString(project.Name) {
			continue
		}
		if !data.IsPersonalSandbox.IsNull() && project.IsPersonalSandbox != data.IsPersonalSandbox.ValueBool() {
			continue
		}
		ids = append(ids, project.ID)
		projects = append(projects, project.toModel())
	}

	idsSet, diags := types.SetValueFrom(ctx, types.StringType, ids)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.IDs = idsSet
	data.Projects = projects

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
