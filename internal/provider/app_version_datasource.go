package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = &AppVersionDataSource{}
	_ datasource.DataSourceWithConfigure = &AppVersionDataSource{}
)

const getAppVersionIDPath = "/provider/apps/get_version_id"

type AppVersionDataSource struct {
	providerData *FlowsProviderConfiguredData
}

func (ds *AppVersionDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	ds.providerData = req.ProviderData.(*FlowsProviderConfiguredData)
}

func NewAppVersionDataSource() datasource.DataSource {
	return &AppVersionDataSource{}
}

type AppVersionDataSourceModel struct {
	ID       types.String `tfsdk:"id"`
	Registry types.String `tfsdk:"registry"`
	Name     types.String `tfsdk:"name"`
	Version  types.String `tfsdk:"version"`
}

func (ds *AppVersionDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_version"
}

func (ds *AppVersionDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Data source for retrieving the application version ID for installing applications.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The computed application version ID, that can be used for installing applications.",
				Computed:    true,
			},
			"registry": schema.StringAttribute{
				Description: "The registry from which to install the application.",
				Optional:    true,
			},
			"name": schema.StringAttribute{
				Description: "The name of the application to install.",
				Required:    true,
			},
			"version": schema.StringAttribute{
				Description: "The version of the application to install. If not provided, the latest version will be used.",
				Optional:    true,
			},
		},
	}
}

type GetAppVersionIDRequest struct {
	Registry string `json:"registry,omitempty"`
	Name     string `json:"name"`
	Version  string `json:"version,omitempty"`
}

type GetAppVersionIDResponse struct {
	ID string `json:"id"`
}

func (ds *AppVersionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AppVersionDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	versionResp, err := CallFlowsAPI[GetAppVersionIDRequest, GetAppVersionIDResponse](*ds.providerData, getAppVersionIDPath, GetAppVersionIDRequest{
		Registry: data.Registry.ValueString(),
		Name:     data.Name.ValueString(),
		Version:  data.Version.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", "Unable to fetch app version id, got error: "+err.Error())
		return
	}

	data.ID = types.StringValue(versionResp.ID)

	// Write state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
