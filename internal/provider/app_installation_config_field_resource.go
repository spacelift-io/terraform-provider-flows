package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &AppInstallationConfigFieldResource{}

type AppInstallationConfigFieldResource struct {
	providerData *FlowsProviderConfiguredData
}

func NewAppInstallationConfigFieldResource() resource.Resource {
	return &AppInstallationConfigFieldResource{}
}

type AppInstallationConfigFieldResourceModel struct {
	AppInstallationID types.String `tfsdk:"app_installation_id"`
	Key               types.String `tfsdk:"key"`
	Value             types.String `tfsdk:"value"`
}

func (r *AppInstallationConfigFieldResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_installation_config_field"
}

func (r *AppInstallationConfigFieldResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Manages app installation's single configuration field.`,
		Attributes: map[string]schema.Attribute{
			"app_installation_id": schema.StringAttribute{
				Description: "ID of the app installation.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"key": schema.StringAttribute{
				MarkdownDescription: "The configuration field key.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"value": schema.StringAttribute{
				MarkdownDescription: `The configuration field value. If "null", the configuration field will be removed.`,
				Optional:            true,
			},
		},
	}
}

func (r *AppInstallationConfigFieldResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	r.providerData = req.ProviderData.(*FlowsProviderConfiguredData)
}

func (r *AppInstallationConfigFieldResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AppInstallationConfigFieldResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, err := CallFlowsAPI[UpdateAppInstallationConfigRequest, UpdateAppInstallationConfigResponse](*r.providerData, updateAppInstallationConfigPath, UpdateAppInstallationConfigRequest{
		ID: data.AppInstallationID.ValueString(),
		ConfigFields: map[string]*string{
			data.Key.ValueString(): data.Value.ValueStringPointer(),
		},
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", "Unable to create app installation config field, got error: "+err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

type GetAppInstallationConfigFieldRequest struct {
	ID  string `json:"id"`
	Key string `json:"key"`
}

type GetAppInstallationConfigFieldResponse struct {
	Value *string `json:"value"`
}

func (r *AppInstallationConfigFieldResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AppInstallationConfigFieldResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	appInstallationID := data.AppInstallationID.ValueString()

	configFieldResp, err := CallFlowsAPI[GetAppInstallationConfigFieldRequest, GetAppInstallationConfigFieldResponse](*r.providerData, getAppInstallationConfigFieldPath, GetAppInstallationConfigFieldRequest{
		ID:  appInstallationID,
		Key: data.Key.ValueString(),
	})
	if err != nil {
		if err.Error() == "not found" {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Client Error", "Unable to read app installation, got error: "+err.Error())
		return
	}

	data.Value = types.StringPointerValue(configFieldResp.Value)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AppInstallationConfigFieldResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AppInstallationConfigFieldResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	var config AppInstallationConfigFieldResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &config)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if !data.Value.Equal(config.Value) {
		_, err := CallFlowsAPI[UpdateAppInstallationConfigRequest, UpdateAppInstallationConfigResponse](*r.providerData, updateAppInstallationConfigPath, UpdateAppInstallationConfigRequest{
			ID: data.AppInstallationID.ValueString(),
			ConfigFields: map[string]*string{
				config.Key.ValueString(): config.Value.ValueStringPointer(),
			},
		})
		if err != nil {
			resp.Diagnostics.AddError("Client Error", "Unable to update app installation config field, got error: "+err.Error())
			return
		}

		data.Value = config.Value
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	}
}

func (r *AppInstallationConfigFieldResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AppInstallationConfigFieldResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := CallFlowsAPI[UpdateAppInstallationConfigRequest, UpdateAppInstallationConfigResponse](*r.providerData, updateAppInstallationConfigPath, UpdateAppInstallationConfigRequest{
		ID: data.AppInstallationID.ValueString(),
		ConfigFields: map[string]*string{
			data.Key.ValueString(): nil,
		},
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", "Unable to delete app installation config field, got error: "+err.Error())
		return
	}
}
