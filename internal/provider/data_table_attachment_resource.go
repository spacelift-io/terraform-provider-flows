package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &DataTableAttachmentResource{}

func NewDataTableAttachmentResource() resource.Resource {
	return &DataTableAttachmentResource{}
}

type DataTableAttachmentResource struct {
	providerData *FlowsProviderConfiguredData
}

type DataTableAttachmentResourceModel struct {
	ID          types.String `tfsdk:"id"`
	DataTableID types.String `tfsdk:"data_table_id"`
	FlowID      types.String `tfsdk:"flow_id"`
}

func (r *DataTableAttachmentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_data_table_attachment"
}

func (r *DataTableAttachmentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Attaches a data table to a flow. When the resource is destroyed, the data table is detached from the flow.`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "ID of the attachment (composite of data_table_id and flow_id).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"data_table_id": schema.StringAttribute{
				Description: "ID of the data table to attach.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"flow_id": schema.StringAttribute{
				Description: "ID of the flow to attach the data table to.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *DataTableAttachmentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	r.providerData = req.ProviderData.(*FlowsProviderConfiguredData)
}

type AttachDataTableToFlowRequest struct {
	DataTableID string `json:"dataTableId"`
	FlowID      string `json:"flowId"`
}

func (r *DataTableAttachmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DataTableAttachmentResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := CallFlowsAPI[AttachDataTableToFlowRequest, struct{}](*r.providerData, "/provider/datatables/attach_to_flow", AttachDataTableToFlowRequest{
		DataTableID: data.DataTableID.ValueString(),
		FlowID:      data.FlowID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", "Unable to attach data table to flow, got error: "+err.Error()+data.DataTableID.ValueString()+data.FlowID.ValueString())
		return
	}

	// Save state
	state := &DataTableAttachmentResourceModel{
		ID:          types.StringValue(fmt.Sprintf("%s/%s", data.DataTableID.ValueString(), data.FlowID.ValueString())),
		DataTableID: data.DataTableID,
		FlowID:      data.FlowID,
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *DataTableAttachmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DataTableAttachmentResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// There's no explicit read endpoint for attachments, so we just keep the state as is
	// If the attachment doesn't exist anymore, the next apply will fail and user can remove it
}

func (r *DataTableAttachmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// This resource doesn't support updates
	resp.Diagnostics.AddError(
		"Update Not Supported",
		`The "data_table_attachment" resource does not support updates. Please destroy and recreate.`,
	)
}

type DetachDataTableFromFlowRequest struct {
	DataTableID string `json:"dataTableId"`
	FlowID      string `json:"flowId"`
}

func (r *DataTableAttachmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DataTableAttachmentResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := CallFlowsAPI[DetachDataTableFromFlowRequest, struct{}](*r.providerData, "/provider/datatables/detach_from_flow", DetachDataTableFromFlowRequest{
		DataTableID: state.DataTableID.ValueString(),
		FlowID:      state.FlowID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to detach data table from flow, got error: %s", err))
		return
	}
}
