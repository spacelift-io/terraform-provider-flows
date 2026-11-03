package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &ProjectAgentPoolAttachmentResource{}
	_ resource.ResourceWithConfigure   = &ProjectAgentPoolAttachmentResource{}
	_ resource.ResourceWithImportState = &ProjectAgentPoolAttachmentResource{}
)

const (
	getProjectAgentPoolAttachmentPath = "/provider/agent_pools/get_project_attachment"
	attachAgentPoolToProjectPath      = "/provider/agent_pools/attach_to_project"
	detachAgentPoolFromProjectPath    = "/provider/agent_pools/detach_from_project"

	// Error messages returned by the Flows API that the resource reacts to.
	errAttachmentNotFound       = "not found"
	errAgentPoolNotExposed      = "agent pool is not exposed to this project"
	errAgentPoolAlreadyAttached = "agent pool is already attached to this project"
)

func NewProjectAgentPoolAttachmentResource() resource.Resource {
	return &ProjectAgentPoolAttachmentResource{}
}

type ProjectAgentPoolAttachmentResource struct {
	providerData *FlowsProviderConfiguredData
}

type ProjectAgentPoolAttachmentResourceModel struct {
	ID          types.String `tfsdk:"id"`
	ProjectID   types.String `tfsdk:"project_id"`
	AgentPoolID types.String `tfsdk:"agent_pool_id"`
}

// ProjectAgentPoolAttachment is an attachment as returned by the Flows provider API.
type ProjectAgentPoolAttachment struct {
	ProjectID   string `json:"projectId"`
	AgentPoolID string `json:"agentPoolId"`
}

func (a ProjectAgentPoolAttachment) toModel() ProjectAgentPoolAttachmentResourceModel {
	return ProjectAgentPoolAttachmentResourceModel{
		ID:          types.StringValue(projectAgentPoolAttachmentID(a.ProjectID, a.AgentPoolID)),
		ProjectID:   types.StringValue(a.ProjectID),
		AgentPoolID: types.StringValue(a.AgentPoolID),
	}
}

// projectAgentPoolAttachmentID builds the composite resource ID, which is also the import ID.
func projectAgentPoolAttachmentID(projectID, agentPoolID string) string {
	return projectID + "/" + agentPoolID
}

// parseProjectAgentPoolAttachmentID splits a composite "<project_id>/<agent_pool_id>" ID.
func parseProjectAgentPoolAttachmentID(id string) (projectID, agentPoolID string, err error) {
	parts := strings.Split(id, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("expected import ID in the format 'project_id/agent_pool_id', got: %q", id)
	}
	return parts[0], parts[1], nil
}

func (r *ProjectAgentPoolAttachmentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_agent_pool_attachment"
}

func (r *ProjectAgentPoolAttachmentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Attaches a private agent pool to a project, so that flows in the project can run on agents from that pool. " +
			"Destroying the resource detaches the pool from the project; neither the project nor the agent pool is deleted.\n\n" +
			"The attachment is identified by `<project_id>/<agent_pool_id>`, which is also the ID to use when importing it. " +
			"Only private agent pools of the organization can be attached: public pools are available to every project. " +
			"Attaching and detaching require an API key with organization admin access, admin access to the project and the `agent_pools:edit` capability; " +
			"reading the attachment only requires viewer access to the project and the `agent_pools:view` capability.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "ID of the attachment (composite of project_id and agent_pool_id).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				Description: "ID of the project to attach the agent pool to.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"agent_pool_id": schema.StringAttribute{
				Description: "ID of the private agent pool to attach.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *ProjectAgentPoolAttachmentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	r.providerData = req.ProviderData.(*FlowsProviderConfiguredData)
}

type ProjectAgentPoolAttachmentRequest struct {
	ProjectID   string `json:"projectId"`
	AgentPoolID string `json:"agentPoolId"`
}

type ProjectAgentPoolAttachmentResponse struct {
	Attachment ProjectAgentPoolAttachment `json:"attachment"`
}

func attachAgentPoolToProject(providerData FlowsProviderConfiguredData, projectID, agentPoolID string) (*ProjectAgentPoolAttachment, error) {
	resp, err := CallFlowsAPI[ProjectAgentPoolAttachmentRequest, ProjectAgentPoolAttachmentResponse](providerData, attachAgentPoolToProjectPath, ProjectAgentPoolAttachmentRequest{
		ProjectID:   projectID,
		AgentPoolID: agentPoolID,
	})
	if err != nil {
		return nil, err
	}
	return &resp.Attachment, nil
}

func getProjectAgentPoolAttachment(providerData FlowsProviderConfiguredData, projectID, agentPoolID string) (*ProjectAgentPoolAttachment, error) {
	resp, err := CallFlowsAPI[ProjectAgentPoolAttachmentRequest, ProjectAgentPoolAttachmentResponse](providerData, getProjectAgentPoolAttachmentPath, ProjectAgentPoolAttachmentRequest{
		ProjectID:   projectID,
		AgentPoolID: agentPoolID,
	})
	if err != nil {
		return nil, err
	}
	return &resp.Attachment, nil
}

func detachAgentPoolFromProject(providerData FlowsProviderConfiguredData, projectID, agentPoolID string) error {
	_, err := CallFlowsAPI[ProjectAgentPoolAttachmentRequest, struct{}](providerData, detachAgentPoolFromProjectPath, ProjectAgentPoolAttachmentRequest{
		ProjectID:   projectID,
		AgentPoolID: agentPoolID,
	})
	return err
}

func (r *ProjectAgentPoolAttachmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ProjectAgentPoolAttachmentResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID, agentPoolID := data.ProjectID.ValueString(), data.AgentPoolID.ValueString()

	attachment, err := attachAgentPoolToProject(*r.providerData, projectID, agentPoolID)
	if err != nil {
		detail := fmt.Sprintf("Unable to attach agent pool to project, got error: %s (project_id=%s, agent_pool_id=%s)", err, projectID, agentPoolID)
		if err.Error() == errAgentPoolAlreadyAttached {
			detail += fmt.Sprintf(". To manage the existing attachment with Terraform, import it using the ID %q.", projectAgentPoolAttachmentID(projectID, agentPoolID))
		}
		resp.Diagnostics.AddError("Client Error", detail)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, attachment.toModel())...)
}

func (r *ProjectAgentPoolAttachmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ProjectAgentPoolAttachmentResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	attachment, err := getProjectAgentPoolAttachment(*r.providerData, state.ProjectID.ValueString(), state.AgentPoolID.ValueString())
	if err != nil {
		if err.Error() == errAttachmentNotFound {
			// The pool is no longer attached to the project (or either side was deleted), remove from state
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read project agent pool attachment, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, attachment.toModel())...)
}

func (r *ProjectAgentPoolAttachmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Both configurable attributes require replacement, so there is nothing to update in-place.
	resp.Diagnostics.AddError(
		"Update Not Supported",
		`The "project_agent_pool_attachment" resource does not support updates. Please destroy and recreate.`,
	)
}

func (r *ProjectAgentPoolAttachmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ProjectAgentPoolAttachmentResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := detachAgentPoolFromProject(*r.providerData, state.ProjectID.ValueString(), state.AgentPoolID.ValueString())
	if err != nil {
		if err.Error() == errAgentPoolNotExposed {
			// Already detached outside of Terraform, nothing left to do
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to detach agent pool from project, got error: %s", err))
		return
	}
}

func (r *ProjectAgentPoolAttachmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	projectID, agentPoolID, err := parseProjectAgentPoolAttachmentID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Import ID", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), projectAgentPoolAttachmentID(projectID, agentPoolID))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), projectID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("agent_pool_id"), agentPoolID)...)
}
