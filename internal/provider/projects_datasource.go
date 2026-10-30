package provider

import (
	"context"
	"strings"

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
	Name            *StringPredicateModel `tfsdk:"name"`
	PersonalSandbox *BoolPredicateModel   `tfsdk:"personal_sandbox"`
	IDs             types.Set             `tfsdk:"ids"`
	Projects        []ProjectModel        `tfsdk:"projects"`
}

// StringPredicateModel requires a string field to equal one of the given values.
type StringPredicateModel struct {
	AnyOf types.List `tfsdk:"any_of"`
}

// BoolPredicateModel requires a boolean field to equal the given value (true when unset).
type BoolPredicateModel struct {
	Equals types.Bool `tfsdk:"equals"`
}

func (p *BoolPredicateModel) value() bool {
	return p.Equals.IsNull() || p.Equals.ValueBool()
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
		MarkdownDescription: "Data source for listing the projects the authentication token has at least viewer access to, optionally narrowed down by predicates. " +
			"All given predicates must match. Projects are returned sorted by name. " +
			"The `ids` attribute is a set of project IDs, so it can be used directly with `for_each`. " +
			"When no project matches, the result is empty rather than an error.",
		Attributes: map[string]schema.Attribute{
			"ids": schema.SetAttribute{
				Description: "Set of the IDs of the projects matching the predicates.",
				Computed:    true,
				ElementType: types.StringType,
			},
			"projects": schema.ListNestedAttribute{
				Description: "List of the projects matching the predicates, sorted by name.",
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
		Blocks: map[string]schema.Block{
			"name": schema.SingleNestedBlock{
				Description: "Require projects to have one of the names.",
				Attributes: map[string]schema.Attribute{
					"any_of": schema.ListAttribute{
						Description: "Project names to match (case-insensitive). At least one is required.",
						Required:    true,
						ElementType: types.StringType,
					},
				},
			},
			"personal_sandbox": schema.SingleNestedBlock{
				Description: "Require projects to be users' personal sandboxes or not.",
				Attributes: map[string]schema.Attribute{
					"equals": schema.BoolAttribute{
						Description: "Whether the project must be a personal sandbox. Defaults to true.",
						Optional:    true,
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

	if data.Name == nil || data.Name.AnyOf.IsNull() || data.Name.AnyOf.IsUnknown() {
		return
	}

	if len(data.Name.AnyOf.Elements()) == 0 {
		resp.Diagnostics.AddAttributeError(path.Root("name").AtName("any_of"), "Empty name predicate", "The name predicate must list at least one project name.")
	}
}

func (ds *ProjectsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ProjectsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var names []string
	if data.Name != nil {
		resp.Diagnostics.Append(data.Name.AnyOf.ElementsAs(ctx, &names, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if len(names) == 0 {
			resp.Diagnostics.AddAttributeError(path.Root("name").AtName("any_of"), "Empty name predicate", "The name predicate must list at least one project name.")
			return
		}
	}

	listResp, err := CallFlowsAPI[ListProjectsRequest, ListProjectsResponse](*ds.providerData, listProjectsPath, ListProjectsRequest{})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", "Unable to list projects, got error: "+err.Error())
		return
	}

	// The API returns every project visible to the token; predicates are applied here.
	ids := make([]string, 0, len(listResp.Projects))
	projects := make([]ProjectModel, 0, len(listResp.Projects))
	for _, project := range listResp.Projects {
		if data.Name != nil && !matchesAnyName(project.Name, names) {
			continue
		}
		if data.PersonalSandbox != nil && project.IsPersonalSandbox != data.PersonalSandbox.value() {
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

// matchesAnyName reports whether name equals one of the candidates, ignoring
// case, which mirrors how project names are kept unique within an organization.
func matchesAnyName(name string, candidates []string) bool {
	for _, candidate := range candidates {
		if strings.EqualFold(name, candidate) {
			return true
		}
	}
	return false
}
