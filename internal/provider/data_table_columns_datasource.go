package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = &DataTableColumnsDataSource{}
	_ datasource.DataSourceWithConfigure = &DataTableColumnsDataSource{}
)

func NewDataTableColumnsDataSource() datasource.DataSource {
	return &DataTableColumnsDataSource{}
}

type DataTableColumnsDataSource struct {
	providerData *FlowsProviderConfiguredData
}

type DataTableColumnsDataSourceModel struct {
	DataTableID types.String `tfsdk:"data_table_id"`
	Columns     types.List   `tfsdk:"columns"`
}

func (ds *DataTableColumnsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_data_table_columns"
}

func (ds *DataTableColumnsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Data source for retrieving all columns of a Data Table.",
		Attributes: map[string]schema.Attribute{
			"data_table_id": schema.StringAttribute{
				Description: "ID of the data table to fetch columns for.",
				Required:    true,
			},
			"columns": schema.ListNestedAttribute{
				Description: "List of columns in the data table.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Description: "ID of the column.",
							Computed:    true,
						},
						"name": schema.StringAttribute{
							Description: "Name of the column.",
							Computed:    true,
						},
						"type": schema.StringAttribute{
							Description: "Type of the column (e.g., boolean, datetime, float, integer, json, row_ref, string).",
							Computed:    true,
						},
						"ref_table_id": schema.StringAttribute{
							Description: "ID of the referenced table (only for row_ref type columns).",
							Computed:    true,
						},
					},
				},
			},
		},
	}
}

func (ds *DataTableColumnsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	ds.providerData = req.ProviderData.(*FlowsProviderConfiguredData)
}

var columnAttrTypes = map[string]attr.Type{
	"id":           types.StringType,
	"name":         types.StringType,
	"type":         types.StringType,
	"ref_table_id": types.StringType,
}

func (ds *DataTableColumnsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DataTableColumnsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	dataTable, err := CallFlowsAPI[ReadDataTableRequest, ReadDataTableResponse](
		*ds.providerData,
		"/provider/datatables/get",
		ReadDataTableRequest{ID: data.DataTableID.ValueString()},
	)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list data table columns, got error: %s", err))
		return
	}

	columnObjects := make([]attr.Value, 0, len(dataTable.DataTable.Columns))
	for _, col := range dataTable.DataTable.Columns {
		refTableID := types.StringNull()
		if col.RefTableID != nil {
			refTableID = types.StringValue(*col.RefTableID)
		}

		obj, diags := types.ObjectValue(columnAttrTypes, map[string]attr.Value{
			"id":           types.StringValue(col.ID),
			"name":         types.StringValue(col.Name),
			"type":         types.StringValue(col.Type),
			"ref_table_id": refTableID,
		})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		columnObjects = append(columnObjects, obj)
	}

	columnsList, diags := types.ListValue(types.ObjectType{AttrTypes: columnAttrTypes}, columnObjects)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.Columns = columnsList

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
