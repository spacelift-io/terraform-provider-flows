resource "flows_data_table" "example" {
  project_id = "your-project-id"
  name       = "my-data-table"
}

data "flows_data_table_columns" "example" {
  data_table_id = flows_data_table.example.id
}
