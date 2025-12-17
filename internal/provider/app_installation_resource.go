package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                     = &AppInstallationResource{}
	_ resource.ResourceWithImportState      = &AppInstallationResource{}
	_ resource.ResourceWithConfigValidators = &AppInstallationResource{}
)

const (
	getInstallationPath            = "/provider/apps/get_installation"
	createInstallationPath         = "/provider/apps/create_installation"
	updateInstallationConfigPath   = "/provider/apps/update_installation_config"
	updateInstallationMetadataPath = "/provider/apps/update_installation_metadata"
	updateInstallationVersionPath  = "/provider/apps/update_installation_version"
	deleteInstallationPath         = "/provider/apps/delete_installation"
	confirmInstallationPath        = "/provider/apps/confirm_installation"
)

func NewAppInstallationResource() resource.Resource {
	return &AppInstallationResource{}
}

type AppInstallationResource struct {
	providerData *FlowsProviderConfiguredData
}

type AppInstallationResourceModel struct {
	ProjectID      types.String `tfsdk:"project_id"`
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	AppVersionID   types.String `tfsdk:"app_version_id"`
	CustomRegistry types.Bool   `tfsdk:"custom_registry"`
	ConfigFields   types.Map    `tfsdk:"config_fields"`
	Confirm        types.Bool   `tfsdk:"confirm"`
	WaitForConfirm types.Bool   `tfsdk:"wait_for_confirm"`
	StyleOverride  types.Object `tfsdk:"style_override"`
}

func (m *AppInstallationResourceModel) ShouldConfirm() bool {
	return m.Confirm.IsNull() || m.Confirm.IsUnknown() || m.Confirm.ValueBool()
}

func (r *AppInstallationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_installation"
}

func (r *AppInstallationResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		confirmWaitValidator{},
	}
}

func (r *AppInstallationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Creates and manages an app installation based on the provided configuration.`,
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				Description: "ID of the project to create the app installation in.",
				Required:    true,
			},
			"id": schema.StringAttribute{
				Description: "ID of the app installation.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Name of the app installation.",
				Required:    true,
			},
			"app_version_id": schema.StringAttribute{
				Description: "Version ID of the app to install. It specifies both, the app and the version.",
				Required:    true,
			},
			"custom_registry": schema.BoolAttribute{
				Description: "Specifies whether the app is from a custom registry.",
				Optional:    true,
			},
			"config_fields": schema.MapAttribute{
				Description: "Configuration settings for the app installation.",
				ElementType: types.StringType,
				Optional:    true,
			},
			"confirm": schema.BoolAttribute{
				Description: "Whether to automatically confirm the app installation in case it is in a draft mode.",
				Optional:    true,
			},
			"wait_for_confirm": schema.BoolAttribute{
				Description: "Whether to wait for the app installation to be confirmed when confirm is true.",
				Optional:    true,
			},
			"style_override": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"icon_url": schema.StringAttribute{
						Description: "URL of the icon to use for the app installation.",
						Optional:    true,
					},
					"color": schema.StringAttribute{
						Description: "Color to use for the app installation in hex format (e.g., #FF5733).",
						Optional:    true,
					},
				},
				Optional: true,
			},
		},
	}
}

func (r *AppInstallationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	r.providerData = req.ProviderData.(*FlowsProviderConfiguredData)
}

type AppInstallationStyleOverride struct {
	IconURL string `json:"iconUrl"`
	Color   string `json:"color"`
}

func NewAppInstallationStyleOverride(data types.Object) *AppInstallationStyleOverride {
	var styleOverride *AppInstallationStyleOverride

	if !data.IsNull() && !data.IsUnknown() {
		styleOverride = &AppInstallationStyleOverride{}

		iconURL, ok := data.Attributes()["icon_url"]
		if ok {
			styleOverride.IconURL = iconURL.(types.String).ValueString()
		}

		color, ok := data.Attributes()["color"]
		if ok {
			styleOverride.Color = color.(types.String).ValueString()
		}
	}

	return styleOverride
}

type CreateAppInstallationRequest struct {
	ProjectID      string                        `json:"projectId"`
	Name           string                        `json:"name"`
	AppVersionID   string                        `json:"appVersionId"`
	CustomRegistry bool                          `json:"customRegistry"`
	StyleOverride  *AppInstallationStyleOverride `json:"styleOverride"`
}

type CreateAppInstallationResponse struct {
	ID    string `json:"id"`
	Draft bool   `json:"draft"`
}

type UpdateAppInstallationConfigRequest struct {
	ID           string            `json:"id"`
	ConfigFields map[string]string `json:"configFields"`
}

type UpdateAppInstallationConfigResponse struct {
	Draft bool `json:"draft"`
}

type ConfirmAppInstallationRequest struct {
	ID string `json:"id"`
}

func (r *AppInstallationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AppInstallationResourceModel

	// Read Terraform plan data into the model.
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createAppInstallationRes, err := CallFlowsAPI[CreateAppInstallationRequest, CreateAppInstallationResponse](*r.providerData, createInstallationPath, CreateAppInstallationRequest{
		ProjectID:      data.ProjectID.ValueString(),
		Name:           data.Name.ValueString(),
		AppVersionID:   data.AppVersionID.ValueString(),
		CustomRegistry: data.CustomRegistry.ValueBool(),
		StyleOverride:  NewAppInstallationStyleOverride(data.StyleOverride),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", "Unable to create app installation, got error: "+err.Error())
		return
	}

	data.ID = types.StringValue(createAppInstallationRes.ID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

	canConfirm := createAppInstallationRes.Draft

	if !data.ConfigFields.IsNull() && !data.ConfigFields.IsUnknown() {
		appInstallation, err := CallFlowsAPI[UpdateAppInstallationConfigRequest, UpdateAppInstallationConfigResponse](*r.providerData, updateInstallationConfigPath, UpdateAppInstallationConfigRequest{
			ID: createAppInstallationRes.ID,
			ConfigFields: func() map[string]string {
				m := make(map[string]string)

				for k, v := range data.ConfigFields.Elements() {
					m[k] = v.(types.String).ValueString()
				}

				return m
			}(),
		})
		if err != nil {
			resp.Diagnostics.AddError("Client Error", "Unable to update app installation config, got error: "+err.Error())
			return
		}

		canConfirm = appInstallation.Draft
	}

	if canConfirm && data.ShouldConfirm() {
		r.Confirm(
			ctx,
			createAppInstallationRes.ID,
			data.WaitForConfirm.ValueBool() || data.WaitForConfirm.IsNull() || data.WaitForConfirm.IsUnknown(),
			&resp.Diagnostics,
		)
	}
}

type GetAppInstallationRequest struct {
	ID string `json:"id"`
}

type GetAppInstallationResponse struct {
	Name          string                        `json:"name"`
	Status        string                        `json:"status"`
	AppVersionID  string                        `json:"appVersionId"`
	StyleOverride *AppInstallationStyleOverride `json:"styleOverride"`
	ConfigFields  map[string]string             `json:"configFields"`
}

func (r *AppInstallationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AppInstallationResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	appInstallation, err := CallFlowsAPI[GetAppInstallationRequest, GetAppInstallationResponse](*r.providerData, getInstallationPath, GetAppInstallationRequest{
		ID: data.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", "Unable to read app installation, got error: "+err.Error())
		return
	}

	data.Name = types.StringValue(appInstallation.Name)
	data.AppVersionID = types.StringValue(appInstallation.AppVersionID)

	if appInstallation.StyleOverride == nil {
		data.StyleOverride = types.ObjectNull(map[string]attr.Type{
			"icon_url": types.StringType,
			"color":    types.StringType,
		})
	} else {
		iconURL := types.StringNull()
		if appInstallation.StyleOverride.IconURL != "" {
			iconURL = types.StringValue(appInstallation.StyleOverride.IconURL)
		}

		color := types.StringNull()
		if appInstallation.StyleOverride.Color != "" {
			color = types.StringValue(appInstallation.StyleOverride.Color)
		}

		data.StyleOverride = types.ObjectValueMust(
			map[string]attr.Type{
				"icon_url": types.StringType,
				"color":    types.StringType,
			},
			map[string]attr.Value{
				"icon_url": iconURL,
				"color":    color,
			},
		)
	}

	if len(appInstallation.ConfigFields) != 0 {
		data.ConfigFields = func() types.Map {
			m := make(map[string]attr.Value)

			for k, v := range appInstallation.ConfigFields {
				m[k] = types.StringValue(v)
			}

			return types.MapValueMust(types.StringType, m)
		}()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

type UpdateAppInstallationMetadataRequest struct {
	ID            string                        `json:"id"`
	Name          string                        `json:"name"`
	StyleOverride *AppInstallationStyleOverride `json:"styleOverride"`
}

type UpdateAppInstallationMetadataResponse struct {
	Draft bool `json:"draft"`
}

type UpdateAppInstallationVersionRequest struct {
	ID             string `json:"id"`
	CustomRegistry bool   `json:"customRegistry"`
	AppVersionID   string `json:"appVersionId"`
}

type UpdateAppInstallationVersionResponse struct {
	Draft bool `json:"draft"`
}

func (r *AppInstallationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AppInstallationResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	var config AppInstallationResourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if !data.CustomRegistry.Equal(config.CustomRegistry) {
		resp.Diagnostics.AddError("Immutable field", "`custom_registry` is an immutable field and cannot be changed after creation. Please recreate the resource to change this value.")
	}

	var canConfirm bool

	if !data.Name.Equal(config.Name) || !data.StyleOverride.Equal(config.StyleOverride) {
		metaResp, err := CallFlowsAPI[UpdateAppInstallationMetadataRequest, UpdateAppInstallationMetadataResponse](*r.providerData, updateInstallationMetadataPath, UpdateAppInstallationMetadataRequest{
			ID:            data.ID.ValueString(),
			Name:          config.Name.ValueString(),
			StyleOverride: NewAppInstallationStyleOverride(config.StyleOverride),
		})
		if err != nil {
			resp.Diagnostics.AddError("Client Error", "Unable to update app installation metadata, got error: "+err.Error())
			return
		}

		data.Name = config.Name
		data.StyleOverride = config.StyleOverride

		canConfirm = metaResp.Draft
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

	if !data.AppVersionID.Equal(config.AppVersionID) {
		versionResp, err := CallFlowsAPI[UpdateAppInstallationVersionRequest, UpdateAppInstallationVersionResponse](*r.providerData, updateInstallationVersionPath, UpdateAppInstallationVersionRequest{
			ID:             data.ID.ValueString(),
			CustomRegistry: config.CustomRegistry.ValueBool(),
			AppVersionID:   config.AppVersionID.ValueString(),
		})
		if err != nil {
			resp.Diagnostics.AddError("Client Error", "Unable to update app installation version, got error: "+err.Error())
			return
		}

		data.AppVersionID = config.AppVersionID
		canConfirm = versionResp.Draft
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

	if !data.ConfigFields.Equal(config.ConfigFields) {
		confResp, err := CallFlowsAPI[UpdateAppInstallationConfigRequest, UpdateAppInstallationConfigResponse](*r.providerData, updateInstallationConfigPath, UpdateAppInstallationConfigRequest{
			ID: data.ID.ValueString(),
			ConfigFields: func() map[string]string {
				m := make(map[string]string)

				for k, v := range config.ConfigFields.Elements() {
					m[k] = v.(types.String).ValueString()
				}

				return m
			}(),
		})
		if err != nil {
			resp.Diagnostics.AddError("Client Error", "Unable to update app installation config, got error: "+err.Error())
			return
		}

		data.ConfigFields = config.ConfigFields
		canConfirm = confResp.Draft
	}

	data.Confirm = config.Confirm
	data.WaitForConfirm = config.WaitForConfirm

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

	if canConfirm && config.ShouldConfirm() {
		r.Confirm(
			ctx,
			data.ID.String(),
			config.WaitForConfirm.ValueBool() || config.WaitForConfirm.IsNull() || config.WaitForConfirm.IsUnknown(),
			&resp.Diagnostics,
		)
	}
}

type DeleteAppInstallationRequest struct {
	ID string `json:"id"`
}

func (r *AppInstallationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AppInstallationResourceModel

	// Read Terraform prior state data into the model.
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Delete the app installation.
	_, err := CallFlowsAPI[DeleteAppInstallationRequest, struct{}](*r.providerData, deleteInstallationPath, DeleteAppInstallationRequest{
		ID: data.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete app installation, got error: %s", err))
		return
	}
}

func (r *AppInstallationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *AppInstallationResource) Confirm(
	ctx context.Context,
	id string,
	wait bool,
	dg *diag.Diagnostics,
) {
	_, err := CallFlowsAPI[ConfirmAppInstallationRequest, struct{}](*r.providerData, confirmInstallationPath, ConfirmAppInstallationRequest{
		ID: id,
	})
	if err != nil {
		dg.AddError("Client Error", fmt.Sprintf("Unable to confirm app installation %q, got error: %s", id, err.Error()))
		return
	}

	if !wait {
		return
	}

	// Poll for the status to settle
	maxRetries := 60 // 5 minutes with 5-second intervals

	retryInterval := 5 * time.Second
	var status string

	for i := range maxRetries {
		appInstallation, err := CallFlowsAPI[GetAppInstallationRequest, GetAppInstallationResponse](
			*r.providerData,
			getInstallationPath,
			GetAppInstallationRequest{
				ID: id,
			},
		)
		if err != nil {
			dg.AddError("Client Error", fmt.Sprintf("Unable to read app installation, got error: %s", err.Error()))
			return
		}

		tflog.Debug(ctx, "App Installation status", map[string]any{
			"app_installation_id": id,
			"status":              appInstallation.Status,
			"attempt":             i + 1,
		})

		status = appInstallation.Status

		switch status {
		case "ready":
			// Success case
			return
		case "failed", "drifted", "draining_failed", "draining", "drained":
			// Terminal failure states
			dg.AddError(
				"App Installation Failed",
				fmt.Sprintf("App Installation %q reached status '%s' instead of 'ready'", id, status),
			)

			return
		case "draft", "in_progress":
			// Transitional states, continue polling
			time.Sleep(retryInterval)
			continue
		default:
			// Unknown status
			dg.AddError(
				"Unknown App Installation Status",
				fmt.Sprintf("App Installation %s has unknown status '%s'", id, status),
			)

			return
		}
	}

	// Timeout reached
	dg.AddError(
		"App Installation Confirmation Timeout",
		fmt.Sprintf("App Installation %s did not reach a settled state within 5 minutes, last status was '%s'", id, status),
	)
}

type confirmWaitValidator struct{}

func (v confirmWaitValidator) Description(ctx context.Context) string {
	return "If wait_for_confirm is true, confirm must also be true."
}

func (v confirmWaitValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v confirmWaitValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg AppInstallationResourceModel

	diags := req.Config.Get(ctx, &cfg)

	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Be defensive about unknown/null values during planning
	if cfg.WaitForConfirm.IsUnknown() || cfg.WaitForConfirm.IsNull() ||
		cfg.Confirm.IsUnknown() || cfg.Confirm.IsNull() {
		return
	}

	if cfg.WaitForConfirm.ValueBool() && !cfg.Confirm.ValueBool() {
		resp.Diagnostics.AddAttributeError(
			path.Root("wait_for_confirm"),
			"Invalid configuration",
			"`wait_for_confirm` can only be true when `confirm` is true.",
		)
	}
}
