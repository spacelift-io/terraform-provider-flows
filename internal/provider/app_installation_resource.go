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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapdefault"
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
	getAppInstallationPath            = "/provider/apps/get_installation"
	getAppInstallationStatusPath      = "/provider/apps/get_installation_status"
	getAppInstallationConfigFieldPath = "/provider/apps/get_installation_config_field"
	createAppInstallationPath         = "/provider/apps/create_installation"
	updateAppInstallationConfigPath   = "/provider/apps/update_installation_config"
	updateAppInstallationMetadataPath = "/provider/apps/update_installation_metadata"
	updateAppInstallationVersionPath  = "/provider/apps/update_installation_version"
	deleteAppInstallationPath         = "/provider/apps/delete_installation"
	confirmAppInstallationPath        = "/provider/apps/confirm_installation"
)

const (
	maxPollRetries    = 60
	pollRetryInterval = 5 * time.Second
)

type AppInstallationResource struct {
	providerData *FlowsProviderConfiguredData
}

func NewAppInstallationResource() resource.Resource {
	return &AppInstallationResource{}
}

type AppInstallationResourceModel struct {
	ProjectID     types.String `tfsdk:"project_id"`
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	App           types.Object `tfsdk:"app"`
	ConfigFields  types.Map    `tfsdk:"config_fields"`
	Confirm       types.Bool   `tfsdk:"confirm"`
	WaitForReady  types.Bool   `tfsdk:"wait_for_ready"`
	StyleOverride types.Object `tfsdk:"style_override"`
}

type AppInstallationApp struct {
	VersionID string `json:"versionId"`
	Custom    bool   `json:"custom"`
}

func (r *AppInstallationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_installation"
}

func (r *AppInstallationResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		waitForReadyValidator{},
	}
}

func (r *AppInstallationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Creates and manages an app installation based on the provided configuration.`,
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				Description: "ID of the project to create the app installation in.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
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
			"app": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"version_id": schema.StringAttribute{
						Description: "Version ID of the app to install. It specifies both, the app and the version.",
						Required:    true,
					},
					"custom": schema.BoolAttribute{
						Description: "Specifies whether the version is from a custom app.",
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(false),
					},
				},
			},
			"config_fields": schema.MapAttribute{
				Description: "Configuration settings for the app installation.",
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Default: mapdefault.StaticValue(
					types.MapValueMust(
						types.StringType,
						make(map[string]attr.Value),
					),
				),
			},
			"confirm": schema.BoolAttribute{
				Description: "Whether to automatically confirm the app installation in case it is in a draft mode.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"wait_for_ready": schema.BoolAttribute{
				Description: `Whether to wait for the app installation to be set to a ready state when "confirm" is true.`,
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
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

	if !data.IsNull() {
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
	ProjectID     string                        `json:"projectId"`
	Name          string                        `json:"name"`
	App           AppInstallationApp            `json:"app"`
	StyleOverride *AppInstallationStyleOverride `json:"styleOverride"`
}

type CreateAppInstallationResponse struct {
	ID    string `json:"id"`
	Draft bool   `json:"draft"`
}

type UpdateAppInstallationConfigRequest struct {
	ID           string             `json:"id"`
	ConfigFields map[string]*string `json:"configFields"`
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
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createAppInstallationRes, err := CallFlowsAPI[CreateAppInstallationRequest, CreateAppInstallationResponse](*r.providerData, createAppInstallationPath, CreateAppInstallationRequest{
		ProjectID: data.ProjectID.ValueString(),
		Name:      data.Name.ValueString(),
		App: AppInstallationApp{
			VersionID: data.App.Attributes()["version_id"].(types.String).ValueString(),
			Custom:    data.App.Attributes()["custom"].(types.Bool).ValueBool(),
		},
		StyleOverride: NewAppInstallationStyleOverride(data.StyleOverride),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", "Unable to create app installation, got error: "+err.Error())
		return
	}

	data.ID = types.StringValue(createAppInstallationRes.ID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

	if len(data.ConfigFields.Elements()) != 0 {
		_, err := CallFlowsAPI[UpdateAppInstallationConfigRequest, UpdateAppInstallationConfigResponse](*r.providerData, updateAppInstallationConfigPath, UpdateAppInstallationConfigRequest{
			ID: createAppInstallationRes.ID,
			ConfigFields: func() map[string]*string {
				m := make(map[string]*string)

				for k, v := range data.ConfigFields.Elements() {
					nv := v.(types.String).ValueString()
					m[k] = &nv
				}

				return m
			}(),
		})
		if err != nil {
			resp.Diagnostics.AddError("Client Error", "Unable to update app installation config, got error: "+err.Error())
			return
		}
	}

	if data.Confirm.ValueBool() {
		ok := ConfirmAppInstallation(
			ctx,
			*r.providerData,
			createAppInstallationRes.ID,
			&resp.Diagnostics,
		)
		if !ok {
			return
		}
	}

	if data.WaitForReady.ValueBool() {
		if !data.Confirm.ValueBool() {
			resp.Diagnostics.AddWarning("Invalid Configuration", `"wait_for_ready" is true but "confirm" is false. Skipping wait for ready.`)
			return
		}

		WaitForAppInstallationReady(
			ctx,
			*r.providerData,
			createAppInstallationRes.ID,
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
	App           AppInstallationApp            `json:"app"`
	StyleOverride *AppInstallationStyleOverride `json:"styleOverride"`
	ConfigFields  map[string]string             `json:"configFields"`
}

func (r *AppInstallationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AppInstallationResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	appInstallation, err := CallFlowsAPI[GetAppInstallationRequest, GetAppInstallationResponse](*r.providerData, getAppInstallationPath, GetAppInstallationRequest{
		ID: data.ID.ValueString(),
	})
	if err != nil {
		if err.Error() == "not found" {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Client Error", "Unable to read app installation, got error: "+err.Error())
		return
	}

	data.Name = types.StringValue(appInstallation.Name)
	data.App = types.ObjectValueMust(
		map[string]attr.Type{
			"version_id": types.StringType,
			"custom":     types.BoolType,
		},
		map[string]attr.Value{
			"version_id": types.StringValue(appInstallation.App.VersionID),
			"custom":     types.BoolValue(appInstallation.App.Custom),
		},
	)

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

	data.ConfigFields = func() types.Map {
		m := make(map[string]attr.Value)

		// Only include fields that were originally in the plan/state.
		for k, v := range appInstallation.ConfigFields {
			_, ok := data.ConfigFields.Elements()[k]
			if !ok {
				continue
			}

			m[k] = types.StringValue(v)
		}

		return types.MapValueMust(types.StringType, m)
	}()

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
	ID  string             `json:"id"`
	App AppInstallationApp `json:"app"`
}

type UpdateAppInstallationVersionResponse struct {
	Draft bool `json:"draft"`
}

func (r *AppInstallationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AppInstallationResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	var config AppInstallationResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &config)...)

	if resp.Diagnostics.HasError() {
		return
	}

	var checksChanged bool

	if !data.Confirm.Equal(config.Confirm) {
		data.Confirm = config.Confirm
		checksChanged = true
	}

	if !data.WaitForReady.Equal(config.WaitForReady) {
		data.WaitForReady = config.WaitForReady
		checksChanged = true
	}

	if checksChanged {
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	}

	var canConfirm bool

	if !data.Name.Equal(config.Name) || !data.StyleOverride.Equal(config.StyleOverride) {
		reqResp, err := CallFlowsAPI[UpdateAppInstallationMetadataRequest, UpdateAppInstallationMetadataResponse](*r.providerData, updateAppInstallationMetadataPath, UpdateAppInstallationMetadataRequest{
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
		canConfirm = reqResp.Draft

		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	}

	if !data.App.Equal(config.App) {
		reqResp, err := CallFlowsAPI[UpdateAppInstallationVersionRequest, UpdateAppInstallationVersionResponse](*r.providerData, updateAppInstallationVersionPath, UpdateAppInstallationVersionRequest{
			ID: data.ID.ValueString(),
			App: AppInstallationApp{
				VersionID: config.App.Attributes()["version_id"].(types.String).ValueString(),
				Custom:    config.App.Attributes()["custom"].(types.Bool).ValueBool(),
			},
		})
		if err != nil {
			resp.Diagnostics.AddError("Client Error", "Unable to update app installation version, got error: "+err.Error())
			return
		}

		data.App = config.App
		canConfirm = reqResp.Draft

		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	}

	if !data.ConfigFields.Equal(config.ConfigFields) {
		if len(config.ConfigFields.Elements()) != 0 {
			reqResp, err := CallFlowsAPI[UpdateAppInstallationConfigRequest, UpdateAppInstallationConfigResponse](*r.providerData, updateAppInstallationConfigPath, UpdateAppInstallationConfigRequest{
				ID: data.ID.ValueString(),
				ConfigFields: func() map[string]*string {
					m := make(map[string]*string)

					for k, v := range config.ConfigFields.Elements() {
						if v.IsNull() {
							m[k] = nil
							continue
						}

						nv := v.(types.String).ValueString()
						m[k] = &nv
					}

					return m
				}(),
			})
			if err != nil {
				resp.Diagnostics.AddError("Client Error", "Unable to update app installation config, got error: "+err.Error())
				return
			}

			canConfirm = reqResp.Draft
		}

		data.ConfigFields = config.ConfigFields
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	}

	if canConfirm && config.Confirm.ValueBool() {
		ok := ConfirmAppInstallation(
			ctx,
			*r.providerData,
			data.ID.ValueString(),
			&resp.Diagnostics,
		)
		if !ok {
			return
		}
	}

	if config.WaitForReady.ValueBool() {
		if !data.Confirm.ValueBool() {
			resp.Diagnostics.AddWarning("Invalid Configuration", `"wait_for_ready" is true but "confirm" is false. Skipping wait for ready.`)
			return
		}

		WaitForAppInstallationReady(
			ctx,
			*r.providerData,
			data.ID.ValueString(),
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
	_, err := CallFlowsAPI[DeleteAppInstallationRequest, struct{}](*r.providerData, deleteAppInstallationPath, DeleteAppInstallationRequest{
		ID: data.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete app installation, got error: %s", err))
		return
	}

	r.WaitForDeleted(ctx, data.ID.ValueString(), &resp.Diagnostics)
}

func (r *AppInstallationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *AppInstallationResource) WaitForDeleted(
	ctx context.Context,
	id string,
	dg *diag.Diagnostics,
) {
	var status string

	for i := range maxPollRetries {
		appInstallation, err := CallFlowsAPI[GetAppInstallationRequest, GetAppInstallationResponse](
			*r.providerData,
			getAppInstallationPath,
			GetAppInstallationRequest{
				ID: id,
			},
		)
		if err != nil {
			if err.Error() == "not found" {
				// Success case
				return
			}

			dg.AddError("Client Error", fmt.Sprintf("Unable to read app installation, got error: %s", err.Error()))
			return
		}

		status = appInstallation.Status

		if status != "draining" && status != "drained" {
			dg.AddError(
				"App Installation Deletion Failed",
				fmt.Sprintf(`App Installation %s reached status %q instead of being deleted`, id, status),
			)

			return
		}

		tflog.Debug(ctx, "App Installation deletion status retry", map[string]any{
			"app_installation_id": id,
			"status":              appInstallation.Status,
			"attempt":             i + 1,
		})

		// Transitional states, continue polling
		time.Sleep(pollRetryInterval)
	}

	// Timeout reached
	dg.AddError(
		"App Installation Deletion Timeout",
		fmt.Sprintf(`App Installation %s did not reach a deleted state within 5 minutes, last status was %q`, id, status),
	)
}

type waitForReadyValidator struct{}

func (v waitForReadyValidator) Description(ctx context.Context) string {
	return `If "wait_for_ready" is true, "confirm" must also be true.`
}

func (v waitForReadyValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v waitForReadyValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg AppInstallationResourceModel

	diags := req.Config.Get(ctx, &cfg)

	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if cfg.WaitForReady.IsUnknown() || cfg.Confirm.IsUnknown() {
		return
	}

	if cfg.WaitForReady.ValueBool() && !cfg.Confirm.ValueBool() {
		resp.Diagnostics.AddAttributeError(
			path.Root("wait_for_ready"),
			"Invalid configuration",
			`"wait_for_ready" can only be true when "confirm" is true.`,
		)
	}
}

func ConfirmAppInstallation(
	ctx context.Context,
	provider FlowsProviderConfiguredData,
	id string,
	dg *diag.Diagnostics,
) bool {
	_, err := CallFlowsAPI[ConfirmAppInstallationRequest, struct{}](provider, confirmAppInstallationPath, ConfirmAppInstallationRequest{
		ID: id,
	})
	if err != nil && err.Error() != "app installation is not a draft" {
		dg.AddError("Client Error", fmt.Sprintf("Unable to confirm app installation %q, got error: %s", id, err.Error()))
		return false
	}

	return true
}

type GetAppInstallationStatusRequest struct {
	ID string `json:"id"`
}

type GetAppInstallationStatusResponse struct {
	Status string `json:"status"`
}

func WaitForAppInstallationReady(
	ctx context.Context,
	provider FlowsProviderConfiguredData,
	id string,
	dg *diag.Diagnostics,
) string {
	var status string

	for i := range maxPollRetries {
		appInstallation, err := CallFlowsAPI[GetAppInstallationStatusRequest, GetAppInstallationStatusResponse](
			provider,
			getAppInstallationPath,
			GetAppInstallationStatusRequest{
				ID: id,
			},
		)
		if err != nil {
			dg.AddError("Client Error", fmt.Sprintf("Unable to read app installation status, got error: %s", err.Error()))
			return ""
		}

		tflog.Debug(ctx, "App Installation confirmation status retry", map[string]any{
			"app_installation_id": id,
			"status":              appInstallation.Status,
			"attempt":             i + 1,
		})

		status = appInstallation.Status

		switch status {
		case "ready":
			// Success case
			return status
		case "failed", "drifted", "draining_failed", "draining", "drained":
			// Terminal failure states
			dg.AddError(
				"App Installation Failed",
				fmt.Sprintf(`App Installation %q reached status %q instead of "ready"`, id, status),
			)

			return status
		case "draft", "in_progress":
			// Transitional states, continue polling
			time.Sleep(pollRetryInterval)
			continue
		default:
			// Unknown status
			dg.AddError(
				"Unknown App Installation Status",
				fmt.Sprintf(`App Installation %s has unknown status %q`, id, status),
			)

			return status
		}
	}

	// Timeout reached
	dg.AddError(
		"App Installation Confirmation Timeout",
		fmt.Sprintf(`App Installation %s did not reach a settled state within 5 minutes, last status was %q`, id, status),
	)

	return ""
}
