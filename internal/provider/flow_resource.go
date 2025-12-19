package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &FlowResource{}
	_ resource.ResourceWithModifyPlan  = &FlowResource{}
	_ resource.ResourceWithImportState = &FlowResource{}
)

func NewFlowResource() resource.Resource {
	return &FlowResource{}
}

type FlowResource struct {
	providerData *FlowsProviderConfiguredData
}

type FlowResourceModel struct {
	ProjectId              types.String `tfsdk:"project_id"`
	Id                     types.String `tfsdk:"id"`
	Name                   types.String `tfsdk:"name"`
	Definition             types.String `tfsdk:"definition"`
	AppInstallationMapping types.Map    `tfsdk:"app_installation_mapping"`
	Blocks                 types.Map    `tfsdk:"blocks"`
}

func (r *FlowResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_flow"
}

func (r *FlowResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Creates and manages a Flow based on the provided definition in YAML format.

The easiest way to get started is to select a couple blocks through the Flows UI and then copy (via ctrl+c / cmd+c) them. You can then paste into a yaml file and use that as the definition.`,
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				Description: "ID of the project to create the flow in.",
				Required:    true,
			},
			"id": schema.StringAttribute{
				Description: "ID of the flow.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Name of the flow.",
				Required:    true,
			},
			"definition": schema.StringAttribute{
				Description: "YAML definition of the flow, easiest to obtain by copying blocks from the Flows UI.",
				Required:    true,
			},
			"app_installation_mapping": schema.MapAttribute{
				Description: "Mapping of app keys to app installation IDs to use when applying the flow definition. Can be used to specify installation ids when they are not provided in the yaml, or to override them.",
				ElementType: types.StringType,
				Optional:    true,
			},
			"blocks": schema.MapAttribute{
				Description: "Map of blocks in the flow, keyed by their names. Each block exposes its ID.",
				Computed:    true,
				ElementType: types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"id": types.StringType,
					},
				},
			},
		},
	}
}

func (r *FlowResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	r.providerData = req.ProviderData.(*FlowsProviderConfiguredData)
}

func (r *FlowResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data FlowResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createFlowRes, err := CallFlowsAPI[CreateFlowRequest, CreateFlowResponse](*r.providerData, "/provider/flows/create", CreateFlowRequest{
		ProjectID: data.ProjectId.ValueString(),
		Name:      data.Name.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", "Unable to create flow, got error: "+err.Error())
		return
	}
	data.Id = types.StringValue(createFlowRes.Flow.ID)
	// Saving id, in case applying config fails.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

	_, err = CallFlowsAPI[ApplyFlowConfigRequest, struct{}](*r.providerData, "/provider/flows/apply_config", ApplyFlowConfigRequest{
		FlowID:                 createFlowRes.Flow.ID,
		Definition:             data.Definition.ValueString(),
		AppInstallationMapping: getAppInstallationMapping(data.AppInstallationMapping),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", "Unable to apply flow definition, got error: "+err.Error())
		return
	}

	// Get the flow details including blocks
	flowDetails, err := r.getFlowDetails(ctx, createFlowRes.Flow.ID)
	if err != nil {
		resp.Diagnostics.AddError("Unable to fetch flow details", err.Error())
		return
	}
	data.Name = flowDetails.Name
	data.Blocks = flowDetails.Blocks

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

type CreateFlowRequest struct {
	ProjectID string `json:"projectId"`
	Name      string `json:"name"`
}

type CreateFlowResponse struct {
	Flow struct {
		ID string `json:"id"`
	} `json:"flow"`
}

type ApplyFlowConfigRequest struct {
	FlowID                 string            `json:"flowId"`
	Definition             string            `json:"definition"`
	AppInstallationMapping map[string]string `json:"appInstallationMapping,omitempty"`
}

type UpdateFlowRequest struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type DeleteFlowRequest struct {
	ID string `json:"id"`
}

type GetFlowRequest struct {
	FlowID string `json:"flowId"`
}

type GetFlowResponse struct {
	Name   string                  `json:"name"`
	Blocks map[string]GetFlowBlock `json:"blocks"`
}

type GetFlowBlock struct {
	ID string `json:"id"`
}

func (r *FlowResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data FlowResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.Definition.IsNull() {
		// There will be a diff anyway in this case.
		return
	}

	// Get the flow details including blocks
	flowDetails, err := r.getFlowDetails(ctx, data.Id.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to fetch flow details", err.Error())
		return
	}
	data.Name = flowDetails.Name
	data.Blocks = flowDetails.Blocks

	planResp, err := CallFlowsAPI[PlanChangesRequest, PlanChangesResponse](*r.providerData, "/provider/flows/plan_changes", PlanChangesRequest{
		FlowID:                 data.Id.ValueString(),
		Definition:             data.Definition.ValueString(),
		AppInstallationMapping: getAppInstallationMapping(data.AppInstallationMapping),
	})
	if err != nil && strings.Contains(err.Error(), "internal error") {
		resp.Diagnostics.AddError("Client Error", "Unable to plan flow changes, got error: "+err.Error())
		return
	}
	if planResp == nil || len(planResp.Plan.Operations) > 0 {
		// Either the flow in the state is broken, or it semantically differs from what's on the server.
		// Either way, we can take the definition from the backend.

		exportRes, err := CallFlowsAPI[ExportFlowDefinitionRequest, ExportFlowDefinitionResponse](*r.providerData, "/provider/flows/export_definition", ExportFlowDefinitionRequest{
			FlowID: data.Id.ValueString(),
		})
		if err != nil {
			resp.Diagnostics.AddError("Client Error", "Unable to fetch flow definition, got error: "+err.Error())
			return
		}
		data.Definition = types.StringValue(exportRes.Definition)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

type ExportFlowDefinitionRequest struct {
	FlowID string `json:"flowId"`
}

type ExportFlowDefinitionResponse struct {
	Definition string `json:"definition"`
}

func (r *FlowResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	var id types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &id)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if id.IsNull() || id.IsUnknown() {
		// Flow not created yet, nothing to do.
		// TODO: Ideally we'd have an endpoint to make a plan "from nothing" just to validate the definition is correct.
		// If the definition and app installation mapping are known, of course.
		return
	}
	var plannedID types.String
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("id"), &plannedID)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if plannedID.IsNull() || plannedID.IsUnknown() {
		// Flow is planned to be deleted or recreated, nothing to do.
		return
	}

	var data FlowResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var config FlowResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if config.Definition.IsUnknown() {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("definition"), config.Definition)...)
		return
	}
	if config.AppInstallationMapping.IsUnknown() {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("app_installation_mapping"), config.AppInstallationMapping)...)
		return
	}

	// Now, with both the definition and app installation mapping known,
	// we make a plan with the definition from *the config*, as it's now available.
	// If there are no changes, we set the planned state to the current state, indicating that semantically nothing changed.
	// If there are changes, we update the planned state to the config.

	planChangesRes, err := CallFlowsAPI[PlanChangesRequest, PlanChangesResponse](*r.providerData, "/provider/flows/plan_changes", PlanChangesRequest{
		FlowID:                 data.Id.ValueString(),
		Definition:             config.Definition.ValueString(),
		AppInstallationMapping: getAppInstallationMapping(config.AppInstallationMapping),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", "Unable to plan flow changes, got error: "+err.Error())
		return
	}

	if len(planChangesRes.Plan.Operations) == 0 {
		// No changes, set the planned state to the current state.
		resp.Diagnostics.Append(resp.Plan.Set(ctx, &data)...)
	} else {
		// There are changes, set the planned state to the config.
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("definition"), config.Definition)...)
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("app_installation_mapping"), config.AppInstallationMapping)...)

		if planChangesRes.ReadablePlan != nil {
			resp.Diagnostics.AddWarning("Flow Changes Planned", *planChangesRes.ReadablePlan)
		}
	}

	return
}

type PlanChangesRequest struct {
	FlowID                 string            `json:"flowId"`
	Definition             string            `json:"definition"`
	AppInstallationMapping map[string]string `json:"appInstallationMapping,omitempty"`
}

type PlanChangesResponse struct {
	Plan struct {
		Operations []struct {
			Type string `json:"type"`
		} `json:"operations"`
	} `json:"plan"`
	ReadablePlan *string `json:"readablePlan,omitempty"`
}

func (r *FlowResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data FlowResourceModel
	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	var config FlowResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, err := CallFlowsAPI[ApplyFlowConfigRequest, struct{}](*r.providerData, "/provider/flows/apply_config", ApplyFlowConfigRequest{
		FlowID:                 data.Id.ValueString(),
		Definition:             config.Definition.ValueString(),
		AppInstallationMapping: getAppInstallationMapping(config.AppInstallationMapping),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", "Unable to apply flow definition, got error: "+err.Error())
		return
	}

	data.Definition = config.Definition
	data.AppInstallationMapping = config.AppInstallationMapping

	// Update flow name if changed
	if !config.Name.Equal(data.Name) {
		_, err = CallFlowsAPI[UpdateFlowRequest, struct{}](*r.providerData, "/provider/flows/update", UpdateFlowRequest{
			ID:   data.Id.ValueString(),
			Name: config.Name.ValueString(),
		})
		if err != nil {
			resp.Diagnostics.AddError("Client Error", "Unable to update flow metadata, got error: "+err.Error())
			return
		}
		data.Name = config.Name
	}

	// Get the flow details including blocks
	flowDetails, err := r.getFlowDetails(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to fetch flow details", err.Error())
		return
	}
	data.Name = flowDetails.Name
	data.Blocks = flowDetails.Blocks

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FlowResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data FlowResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Delete the flow
	_, err := CallFlowsAPI[DeleteFlowRequest, struct{}](*r.providerData, "/provider/flows/delete", DeleteFlowRequest{
		ID: data.Id.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete flow, got error: %s", err))
		return
	}
}

func (r *FlowResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func getAppInstallationMapping(m types.Map) map[string]string {
	if m.IsNull() || m.IsUnknown() {
		return nil
	}
	result := make(map[string]string, 0)
	for k, v := range m.Elements() {
		if v.IsNull() || v.IsUnknown() {
			continue
		}
		result[k] = v.(types.String).ValueString()
	}
	return result
}

type flowDetailsResult struct {
	Name   types.String
	Blocks types.Map
}

func (r *FlowResource) getFlowDetails(ctx context.Context, flowID string) (*flowDetailsResult, error) {
	getFlowResp, err := CallFlowsAPI[GetFlowRequest, GetFlowResponse](*r.providerData, "/provider/flows/get", GetFlowRequest{
		FlowID: flowID,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to get flow details: %w", err)
	}

	// Convert blocks to Terraform types
	blockElements := make(map[string]attr.Value)
	for name, block := range getFlowResp.Blocks {
		blockAttrs := map[string]attr.Value{
			"id": types.StringValue(block.ID),
		}
		blockObj, _ := types.ObjectValue(
			map[string]attr.Type{
				"id": types.StringType,
			},
			blockAttrs,
		)
		blockElements[name] = blockObj
	}

	blockElementType := types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"id": types.StringType,
		},
	}
	blocks, _ := types.MapValue(blockElementType, blockElements)

	return &flowDetailsResult{
		Name:   types.StringValue(getFlowResp.Name),
		Blocks: blocks,
	}, nil
}
