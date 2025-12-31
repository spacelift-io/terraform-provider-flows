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
var _ resource.Resource = &AppInstallationWaitForReadyResource{}

type AppInstallationWaitForReadyResource struct {
	providerData *FlowsProviderConfiguredData
}

func NewAppInstallationWaitForReadyResource() resource.Resource {
	return &AppInstallationWaitForReadyResource{}
}

type AppInstallationWaitForReadyResourceModel struct {
	AppInstallationID types.String `tfsdk:"app_installation_id"`
	Status            types.String `tfsdk:"status"`
}

func (r *AppInstallationWaitForReadyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_installation_wait_for_ready"
}

func (r *AppInstallationWaitForReadyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Waits for app installation to reach a "ready" state.
This is useful for creating app installations using the "app_installation" resource, and then waiting for it using this resource.
You must ensure that either app installation "confirm" is set to true or you are using "app_installation_confirmation" for the confirmation process to begin.`,
		Attributes: map[string]schema.Attribute{
			"app_installation_id": schema.StringAttribute{
				Description: "ID of the app installation.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: `The final status of the app installation after waiting for it to be "ready".`,
				Computed:            true,
			},
		},
	}
}

func (r *AppInstallationWaitForReadyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	r.providerData = req.ProviderData.(*FlowsProviderConfiguredData)
}

func (r *AppInstallationWaitForReadyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AppInstallationWaitForReadyResourceModel

	// Read Terraform plan data into the model.
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	appInstallationID := data.AppInstallationID.ValueString()

	status := WaitForAppInstallationReady(
		ctx,
		*r.providerData,
		appInstallationID,
		&resp.Diagnostics,
	)
	if status == nil {
		return
	}

	data.Status = types.StringValue(*status)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AppInstallationWaitForReadyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AppInstallationWaitForReadyResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	appInstallationID := data.AppInstallationID.ValueString()

	statusResp, err := CallFlowsAPI[GetAppInstallationStatusRequest, GetAppInstallationStatusResponse](*r.providerData, getAppInstallationStatusPath, GetAppInstallationStatusRequest{
		ID: appInstallationID,
	})
	if err != nil {
		if err.Error() == "not found" {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Client Error", "Unable to read app installation status, got error: "+err.Error())
		return
	}

	data.Status = types.StringValue(statusResp.Status)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AppInstallationWaitForReadyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// This resource doesn't support updates
	resp.Diagnostics.AddError(
		"Update Not Supported",
		`The "app_installation_wait_for_ready" resource does not support updates. Please destroy and recreate.`,
	)
}

func (r *AppInstallationWaitForReadyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Nothing to do on delete.
}
