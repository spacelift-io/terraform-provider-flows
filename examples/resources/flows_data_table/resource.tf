resource "flows_data_table" "example" {
  project_id = "your-project-id"
  name       = "my-data-table"
}

output "data_table_id" {
  value = flows_data_table.example.id
}
