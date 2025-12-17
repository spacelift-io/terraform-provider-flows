package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &SecretResource{}
var _ resource.ResourceWithImportState = &SecretResource{}

func NewSecretResource() resource.Resource {
	return &SecretResource{}
}

type SecretResource struct {
	providerData *FlowsProviderConfiguredData
}

type SecretResourceModel struct {
	Id        types.String `tfsdk:"id"`
	ProjectId types.String `tfsdk:"project_id"`
	Key       types.String `tfsdk:"key"`
	Value     types.String `tfsdk:"value"`
}

func (r *SecretResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_secret"
}

func (r *SecretResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Creates and manages a Project Secret.`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "ID of the secret (composite of project_id and key).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				Description: "`ID of the project to create the secret in.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"key": schema.StringAttribute{
				Description: "Secret key.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"value": schema.StringAttribute{
				Description: "Secret value.",
				Required:    true,
				Sensitive:   true,
			},
		},
	}
}

func (r *SecretResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	r.providerData = req.ProviderData.(*FlowsProviderConfiguredData)
}

type CreateSecretRequest struct {
	ProjectId string `json:"projectId"`
	Key       string `json:"key"`
	Value     string `json:"value"`
}

type CreateSecretResponse struct {
	Data struct {
		Secret struct {
			Key       string    `json:"key"`
			UpdatedAt time.Time `json:"updatedAt"`
		} `json:"secret"`
	} `json:"data"`
}

func (r *SecretResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var config SecretResourceModel

	// Read Terraform plan config into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := CallFlowsAPI[CreateSecretRequest, CreateSecretResponse](*r.providerData, "/provider/organization/create_secret", CreateSecretRequest{
		ProjectId: config.ProjectId.ValueString(),
		Key:       config.Key.ValueString(),
		Value:     config.Value.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", "Unable to create secret, got error: "+err.Error())
		return
	}

	// Set the ID as a composite of project_id and key
	config.Id = types.StringValue(fmt.Sprintf("%s/%s", config.ProjectId.ValueString(), config.Key.ValueString()))

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

type UpdateSecretRequest struct {
	ProjectId string `json:"projectId"`
	Key       string `json:"key"`
	Value     string `json:"value"`
}

func (r *SecretResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state SecretResourceModel
	// Read Terraform prior state into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	var config SecretResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, err := CallFlowsAPI[UpdateSecretRequest, struct{}](*r.providerData, "/provider/organization/update_secret", UpdateSecretRequest{
		ProjectId: config.ProjectId.ValueString(),
		Key:       config.Key.ValueString(),
		Value:     config.Value.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", "Unable to update secret, got error: "+err.Error())
		return
	}

	// Update the ID in case the key or project_id changed
	config.Id = types.StringValue(fmt.Sprintf("%s/%s", config.ProjectId.ValueString(), config.Key.ValueString()))

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

type ReadSecretRequest struct {
	ProjectId string `json:"projectId"`
	Key       string `json:"key"`
}

type ReadSecretResponse struct {
	Data struct {
		Secret struct {
			Key       string    `json:"key"`
			UpdatedAt time.Time `json:"updatedAt"`
		} `json:"secret"`
	} `json:"data"`
	Error string `json:"error"`
}

func (r *SecretResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state SecretResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := CallFlowsAPI[ReadSecretRequest, ReadSecretResponse](*r.providerData, "/provider/organization/read_secret", ReadSecretRequest{
		ProjectId: state.ProjectId.ValueString(),
		Key:       state.Key.ValueString(),
	})
	if err != nil {
		if err.Error() == "not found" {
			// Secret deleted, remove from state
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read secret, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

type DeleteSecretRequest struct {
	ProjectId string `json:"projectId"`
	Key       string `json:"key"`
}

func (r *SecretResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state SecretResourceModel

	// Read Terraform prior  state into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := CallFlowsAPI[DeleteSecretRequest, struct{}](*r.providerData, "/provider/organization/delete_secret", DeleteSecretRequest{
		ProjectId: state.ProjectId.ValueString(),
		Key:       state.Key.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete flow, got error: %s", err))
		return
	}
}

func (r *SecretResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Parse ID with format "project_id/key"
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected import ID in the format 'project_id/key', got: %s", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("key"), parts[1])...)
}
