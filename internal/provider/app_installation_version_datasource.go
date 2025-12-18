package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource                     = &AppVersionDataSource{}
	_ datasource.DataSourceWithConfigure        = &AppVersionDataSource{}
	_ datasource.DataSourceWithConfigValidators = &AppVersionDataSource{}
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
	Registry     types.String `tfsdk:"registry"`
	AppName      types.String `tfsdk:"app_name"`
	AppVersion   types.String `tfsdk:"app_version"`
	Custom       types.Bool   `tfsdk:"custom"`
	AppVersionID types.String `tfsdk:"app_version_id"`
}

func (ds *AppVersionDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_version"
}

func (ds *AppVersionDataSource) ConfigValidators(ctx context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		registryValidator{},
	}
}

func (ds *AppVersionDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"registry": schema.StringAttribute{
				Description: "The registry from which to install the application.",
				Optional:    true,
			},
			"app_name": schema.StringAttribute{
				Description: "The name of the application to install.",
				Required:    true,
			},
			"app_version": schema.StringAttribute{
				Description: "The version of the application to install. If not provided, the latest version will be used.",
				Optional:    true,
			},
			"custom": schema.BoolAttribute{
				Description: "Should specify ture if the application is custom.",
				Optional:    true,
			},
			"app_version_id": schema.StringAttribute{
				Description: "The computed application version ID, that can be used for installing applications.",
				Computed:    true,
			},
		},
	}
}

type GetAppVersionIDRequest struct {
	Registry   string `json:"registry,omitempty"`
	AppName    string `json:"app_name"`
	AppVersion string `json:"app_version,omitempty"`
	Custom     bool   `json:"custom,omitempty"`
}

type GetAppVersionIDResponse struct {
	ID string `json:"id"`
}

func (ds *AppVersionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AppVersionDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	appVersionID, err := CallFlowsAPI[GetAppVersionIDRequest, GetAppVersionIDResponse](*ds.providerData, getAppVersionIDPath, GetAppVersionIDRequest{
		Registry:   data.Registry.ValueString(),
		AppName:    data.AppName.ValueString(),
		AppVersion: data.AppVersion.ValueString(),
		Custom:     data.Custom.ValueBool(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", "Unable to read fetch app version id, got error: "+err.Error())
		return
	}

	data.AppVersionID = types.StringValue(appVersionID.ID)

	// Write state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

type registryValidator struct{}

func (v registryValidator) Description(ctx context.Context) string {
	return "If wait_for_confirm is true, confirm must also be true."
}

func (v registryValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v registryValidator) ValidateDataSource(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	var cfg AppVersionDataSourceModel

	diags := req.Config.Get(ctx, &cfg)

	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !cfg.Custom.ValueBool() {
		return
	}

	if !cfg.Registry.IsUnknown() && !cfg.Registry.IsNull() && cfg.Registry.ValueString() != "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("registry"),
			"Invalid configuration",
			"`registry` can only be specified when `custom` is false.",
		)
	}
}
