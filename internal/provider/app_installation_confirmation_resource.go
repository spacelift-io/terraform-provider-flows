package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &AppInstallationConfirmationResource{}

type AppInstallationConfirmationResource struct {
	providerData *FlowsProviderConfiguredData
}

func NewAppInstallationConfirmationResource() resource.Resource {
	return &AppInstallationConfirmationResource{}
}

type AppInstallationConfirmationResourceModel struct {
	AppInstallationID types.String `tfsdk:"app_installation_id"`
	Status            types.String `tfsdk:"status"`
	WaitForReady      types.Bool   `tfsdk:"wait_for_ready"`
}

func (r *AppInstallationConfirmationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_installation_confirmation"
}

func (r *AppInstallationConfirmationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Confirms an app installation and optionally waits for it to reach a "ready" state.
This is useful for creating app installations using the "app_installation" resource, and then confirming it using this resource.`,
		Attributes: map[string]schema.Attribute{
			"app_installation_id": schema.StringAttribute{
				Description: "ID of the app installation.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "The final status of the app installation after confirmation.",
				Computed:            true,
			},
			"wait_for_ready": schema.BoolAttribute{
				Description: `Whether to wait for the app installation to be set to a "ready" state.`,
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
		},
	}
}

func (r *AppInstallationConfirmationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	r.providerData = req.ProviderData.(*FlowsProviderConfiguredData)
}

func (r *AppInstallationConfirmationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AppInstallationConfirmationResourceModel

	// Read Terraform plan data into the model.
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
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

	if statusResp.Status != "draft" {
		tflog.Info(ctx, "App installation is not a draft, skipping confirmation", map[string]any{
			"app_installation_id": appInstallationID,
			"status":              statusResp.Status,
		})

		return
	}

	ok := ConfirmAppInstallation(ctx, *r.providerData, appInstallationID, &resp.Diagnostics)
	if !ok {
		return
	}

	if data.WaitForReady.ValueBool() {
		status := WaitForAppInstallationReady(
			ctx,
			*r.providerData,
			appInstallationID,
			&resp.Diagnostics,
		)
		if status != "" {
			data.Status = types.StringValue(status)
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		}
	}
}

func (r *AppInstallationConfirmationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AppInstallationConfirmationResourceModel

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

func (r *AppInstallationConfirmationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// This resource doesn't support updates
	resp.Diagnostics.AddError(
		"Update Not Supported",
		`The "app_installation_confirmation" resource does not support updates. Please destroy and recreate.`,
	)
}

func (r *AppInstallationConfirmationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Nothing to do on delete.
}
