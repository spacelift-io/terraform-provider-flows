package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &EntityConfirmationResource{}

func NewEntityConfirmationResource() resource.Resource {
	return &EntityConfirmationResource{}
}

// EntityConfirmationResource defines the resource implementation.
type EntityConfirmationResource struct {
	providerData *FlowsProviderConfiguredData
}

// EntityConfirmationResourceModel describes the resource data model.
type EntityConfirmationResourceModel struct {
	EntityId types.String `tfsdk:"entity_id"`
	Status   types.String `tfsdk:"status"`
}

func (r *EntityConfirmationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_entity_confirmation"
}

func (r *EntityConfirmationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Confirms an entity and waits for it to reach a settled state.

This is useful for creating stateful blocks and entities using the flow resource, and then confirming them using entity_confirmation.`,
		Attributes: map[string]schema.Attribute{
			"entity_id": schema.StringAttribute{
				MarkdownDescription: "The UUID of the entity to confirm",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "The final status of the entity after confirmation",
				Computed:            true,
			},
		},
	}
}

func (r *EntityConfirmationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	r.providerData = req.ProviderData.(*FlowsProviderConfiguredData)
}

type ConfirmEntityLifecycleRequest struct {
	ID string `json:"id"`
}

type GetEntityLifecycleStatusRequest struct {
	EntityID string `json:"entityId"`
}

type GetEntityLifecycleStatusResponse struct {
	Status string `json:"status"`
}

func (r *EntityConfirmationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data EntityConfirmationResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	entityID := data.EntityId.ValueString()

	// First check the current status
	statusResp, err := CallFlowsAPI[GetEntityLifecycleStatusRequest, GetEntityLifecycleStatusResponse](*r.providerData, "/provider/flows/get_entity_lifecycle_status", GetEntityLifecycleStatusRequest{
		EntityID: entityID,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to get entity status, got error: %s", err))
		return
	}

	// Only confirm if the entity is in draft state
	if statusResp.Status == "draft" {
		tflog.Info(ctx, "Confirming entity", map[string]interface{}{
			"entity_id": entityID,
		})

		_, err := CallFlowsAPI[ConfirmEntityLifecycleRequest, struct{}](*r.providerData, "/provider/flows/confirm_entity_lifecycle", ConfirmEntityLifecycleRequest{
			ID: entityID,
		})
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to confirm entity, got error: %s", err))
			return
		}
	} else {
		tflog.Info(ctx, "Entity not a draft, skipping confirmation", map[string]interface{}{
			"entity_id": entityID,
			"status":    statusResp.Status,
		})
	}

	// Poll for the status to settle
	maxRetries := 60 // 5 minutes with 5-second intervals
	retryInterval := 5 * time.Second
	var finalStatus string

	for i := 0; i < maxRetries; i++ {
		statusResp, err := CallFlowsAPI[GetEntityLifecycleStatusRequest, GetEntityLifecycleStatusResponse](*r.providerData, "/provider/flows/get_entity_lifecycle_status", GetEntityLifecycleStatusRequest{
			EntityID: entityID,
		})
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to get entity status, got error: %s", err))
			return
		}

		tflog.Debug(ctx, "Entity status", map[string]interface{}{
			"entity_id": entityID,
			"status":    statusResp.Status,
			"attempt":   i + 1,
		})

		finalStatus = statusResp.Status

		// Check if the status is settled
		switch finalStatus {
		case "ready":
			// Success case
			data.Status = types.StringValue(finalStatus)
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		case "failed", "drifted", "draining_failed", "draining", "drained":
			// Terminal failure states
			resp.Diagnostics.AddError(
				"Entity Confirmation Failed",
				fmt.Sprintf("Entity %q reached status '%s' instead of 'ready'", entityID, finalStatus),
			)
			return
		case "draft", "in_progress":
			// Transitional states, continue polling
			time.Sleep(retryInterval)
			continue
		default:
			// Unknown status
			resp.Diagnostics.AddError(
				"Unknown Entity Status",
				fmt.Sprintf("Entity %s has unknown status '%s'", entityID, finalStatus),
			)
			return
		}
	}

	// Timeout reached
	resp.Diagnostics.AddError(
		"Entity Confirmation Timeout",
		fmt.Sprintf("Entity %s did not reach a settled state within 5 minutes, last status was '%s'", entityID, finalStatus),
	)
}

func (r *EntityConfirmationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data EntityConfirmationResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get current status
	statusResp, err := CallFlowsAPI[GetEntityLifecycleStatusRequest, GetEntityLifecycleStatusResponse](*r.providerData, "/provider/flows/get_entity_lifecycle_status", GetEntityLifecycleStatusRequest{
		EntityID: data.EntityId.ValueString(),
	})
	if err != nil {
		// If we can't read the status, remove from state
		resp.State.RemoveResource(ctx)
		return
	}

	data.Status = types.StringValue(statusResp.Status)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *EntityConfirmationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// This resource doesn't support updates
	resp.Diagnostics.AddError(
		"Update Not Supported",
		"The entity_confirmation resource does not support updates. Please destroy and recreate.",
	)
}

func (r *EntityConfirmationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Nothing to do on delete - this resource is purely for confirmation
	// The entity itself is managed elsewhere
}
