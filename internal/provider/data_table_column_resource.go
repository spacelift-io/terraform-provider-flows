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
var _ resource.Resource = &DataTableColumnResource{}
var _ resource.ResourceWithImportState = &DataTableColumnResource{}

func NewDataTableColumnResource() resource.Resource {
	return &DataTableColumnResource{}
}

type DataTableColumnResource struct {
	providerData *FlowsProviderConfiguredData
}

type DataTableColumnResourceModel struct {
	ID          types.String `tfsdk:"id"`
	DataTableID types.String `tfsdk:"data_table_id"`
	Name        types.String `tfsdk:"name"`
	Type        types.String `tfsdk:"type"`
	RefTableID  types.String `tfsdk:"ref_table_id"`
}

func (r *DataTableColumnResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_data_table_column"
}

func (r *DataTableColumnResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Creates and manages a Data Table Column resource.`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "ID of the data table column.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"data_table_id": schema.StringAttribute{
				Description: "ID of the data table to create the column in.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Name of the column.",
				Required:    true,
			},
			"type": schema.StringAttribute{
				Description: "Type of the column (e.g., boolean, datetime, float, integer, json, row_ref, string).",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"ref_table_id": schema.StringAttribute{
				Description: "ID of the referenced table (only for row_ref type columns).",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *DataTableColumnResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	r.providerData = req.ProviderData.(*FlowsProviderConfiguredData)
}

type CreateDataTableColumnRequest struct {
	DataTableID string  `json:"dataTableId"`
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	RefTableID  *string `json:"refTableId,omitempty"`
}

type CreateDataTableColumnResponse struct {
	Column struct {
		ID string `json:"id"`
	} `json:"column"`
}

func (r *DataTableColumnResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DataTableColumnResourceModel

	// Read Terraform plan config into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := CreateDataTableColumnRequest{
		DataTableID: data.DataTableID.ValueString(),
		Name:        data.Name.ValueString(),
		Type:        data.Type.ValueString(),
	}

	if !data.RefTableID.IsNull() && !data.RefTableID.IsUnknown() {
		refTableID := data.RefTableID.ValueString()
		createReq.RefTableID = &refTableID
	}

	createResp, err := CallFlowsAPI[CreateDataTableColumnRequest, CreateDataTableColumnResponse](*r.providerData, "/provider/datatables/create_datatable_column", createReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", "Unable to create data table column, got error: "+err.Error())
		return
	}

	// Save state
	state := &DataTableColumnResourceModel{
		ID:          types.StringValue(createResp.Column.ID),
		DataTableID: data.DataTableID,
		Name:        data.Name,
		Type:        data.Type,
		RefTableID:  data.RefTableID,
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

type UpdateDataTableColumnRequest struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type UpdateDataTableColumnResponse struct {
	Column struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"column"`
}

func (r *DataTableColumnResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state DataTableColumnResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var config DataTableColumnResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateResp, err := CallFlowsAPI[UpdateDataTableColumnRequest, UpdateDataTableColumnResponse](*r.providerData, "/provider/datatables/update_datatable_column", UpdateDataTableColumnRequest{
		ID:   state.ID.ValueString(),
		Name: config.Name.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", "Unable to update data table column, got error: "+err.Error())
		return
	}

	state.Name = types.StringValue(updateResp.Column.Name)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

type ReadDataTableColumnRequest struct {
	ID string `json:"id"`
}

type ReadDataTableColumnResponse struct {
	Column struct {
		ID         string  `json:"id"`
		Name       string  `json:"name"`
		Type       string  `json:"type"`
		RefTableID *string `json:"refTableId,omitempty"`
	} `json:"column"`
}

func (r *DataTableColumnResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DataTableColumnResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readResp, err := CallFlowsAPI[ReadDataTableColumnRequest, ReadDataTableColumnResponse](*r.providerData, "/provider/datatables/get_datatable_column", ReadDataTableColumnRequest{
		ID: state.ID.ValueString(),
	})
	if err != nil {
		if err.Error() == "not found" {
			// Data table column deleted, remove from state
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read data table column, got error: %s", err))
		return
	}

	state.Name = types.StringValue(readResp.Column.Name)
	state.Type = types.StringValue(readResp.Column.Type)
	if readResp.Column.RefTableID != nil {
		state.RefTableID = types.StringValue(*readResp.Column.RefTableID)
	} else {
		state.RefTableID = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

type DeleteDataTableColumnRequest struct {
	ID string `json:"id"`
}

func (r *DataTableColumnResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DataTableColumnResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := CallFlowsAPI[DeleteDataTableColumnRequest, struct{}](*r.providerData, "/provider/datatables/delete_datatable_column", DeleteDataTableColumnRequest{
		ID: state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete data table column, got error: %s", err))
		return
	}
}

func (r *DataTableColumnResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
