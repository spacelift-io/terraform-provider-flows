package provider

import (
	"context"
	"net/url"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = &FlowsProvider{}

type FlowsProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

type FlowsProviderModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	Token    types.String `tfsdk:"token"`
}

type FlowsProviderConfiguredData struct {
	Endpoint string
	Token    string
}

func (p *FlowsProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "flows"
	resp.Version = p.version
}

func (p *FlowsProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "The Flows endpoint to use. Usually useflows.eu or useflows.us.",
				Required:            true,
			},
			"token": schema.StringAttribute{
				MarkdownDescription: "The authentication token for the Flows API. You may also set this using the FLOWS_TOKEN environment variable. You can get this token by running `flowctl auth token`.",
				Sensitive:           true,
				Optional:            true,
			},
		},
	}
}

func (p *FlowsProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data FlowsProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	token := data.Token.ValueString()
	if token == "" {
		var ok bool
		token, ok = os.LookupEnv("FLOWS_TOKEN")
		if !ok {
			resp.Diagnostics.AddError("Missing FLOWS_TOKEN environment variable.", "The FLOWS_TOKEN environment variable must be set to authenticate requests. Get it by running `flowctl auth token`.")
			return
		}
	}

	endpointParsed, err := url.Parse(data.Endpoint.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid endpoint URL.", "The provided endpoint URL is invalid: "+err.Error())
		return
	}
	if endpointParsed.Scheme == "" {
		endpointParsed.Scheme = "https"
	}

	resp.ResourceData = &FlowsProviderConfiguredData{
		Token:    token,
		Endpoint: endpointParsed.String(),
	}
}

func (p *FlowsProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewFlowResource,
		NewEntityConfirmationResource,
	}
}

func (p *FlowsProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &FlowsProvider{
			version: version,
		}
	}
}
