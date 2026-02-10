package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &DataTableResource{}
var _ resource.ResourceWithImportState = &DataTableResource{}

func NewDataTableResource() resource.Resource {
	return &DataTableResource{}
}

type DataTableResource struct {
	providerData *FlowsProviderConfiguredData
}

type DataTableResourceModel struct {
	ID        types.String `tfsdk:"id"`
	ProjectID types.String `tfsdk:"project_id"`
	Name      types.String `tfsdk:"name"`
}

func (r *DataTableResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_data_table"
}

func (r *DataTableResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Creates and manages a Data Table resource.`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "ID of the data table.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				Description: "ID of the project to create the data table in.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Name of the data table.",
				Required:    true,
			},
		},
	}
}

func (r *DataTableResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	r.providerData = req.ProviderData.(*FlowsProviderConfiguredData)
}

type CreateDataTableRequest struct {
	ProjectID string `json:"projectId"`
	Name      string `json:"name"`
}

type CreateDataTableResponse struct {
	DataTable struct {
		ID string `json:"id"`
	} `json:"dataTable"`
}

func (r *DataTableResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DataTableResourceModel

	// Read Terraform plan config into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createResp, err := CallFlowsAPI[CreateDataTableRequest, CreateDataTableResponse](*r.providerData, "/provider/datatables/create", CreateDataTableRequest{
		ProjectID: data.ProjectID.ValueString(),
		Name:      data.Name.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", "Unable to create data table, got error: "+err.Error())
		return
	}
	// Save state
	state := &DataTableResourceModel{
		ID:        types.StringValue(createResp.DataTable.ID),
		ProjectID: data.ProjectID,
		Name:      data.Name,
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

type UpdateDataTableRequest struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type UpdateDataTableResponse struct {
	DataTable struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"dataTable"`
}

func (r *DataTableResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state DataTableResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var config DataTableResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateResp, err := CallFlowsAPI[UpdateDataTableRequest, UpdateDataTableResponse](*r.providerData, "/provider/datatables/update", UpdateDataTableRequest{
		ID:   state.ID.ValueString(),
		Name: config.Name.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", "Unable to update data table, got error: "+err.Error())
		return
	}

	state.Name = types.StringValue(updateResp.DataTable.Name)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

type ReadDataTableRequest struct {
	ID string `json:"id"`
}

type ReadDataTableResponse struct {
	DataTable struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"dataTable"`
}

func (r *DataTableResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DataTableResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readResp, err := CallFlowsAPI[ReadDataTableRequest, ReadDataTableResponse](*r.providerData, "/provider/datatables/get", ReadDataTableRequest{
		ID: state.ID.ValueString(),
	})
	if err != nil {
		if err.Error() == "not found" {
			// Data table deleted, remove from state
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read data table, got error: %s", err))
		return
	}

	state.Name = types.StringValue(readResp.DataTable.Name)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

type DeleteDataTableRequest struct {
	ID string `json:"id"`
}

func (r *DataTableResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DataTableResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := CallFlowsAPI[DeleteDataTableRequest, struct{}](*r.providerData, "/provider/datatables/delete", DeleteDataTableRequest{
		ID: state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete data table, got error: %s", err))
		return
	}
}

func (r *DataTableResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
