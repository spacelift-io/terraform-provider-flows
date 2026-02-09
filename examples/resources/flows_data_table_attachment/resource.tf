resource "flows_data_table" "example" {
  project_id = "your-project-id"
  name       = "my-data-table"
}

resource "flows_flow" "example" {
  project_id = "your-project-id"
  name       = "my-flow"
  definition = file("${path.module}/flow.yaml")
}

# Attach data table to flow
resource "flows_data_table_attachment" "example" {
  data_table_id = flows_data_table.example.id
  flow_id       = flows_flow.example.id
}

# Attach another data table to the same flow
resource "flows_data_table" "users_table" {
  project_id = "your-project-id"
  name       = "users-table"
}

resource "flows_data_table_attachment" "users_attachment" {
  data_table_id = flows_data_table.users_table.id
  flow_id       = flows_flow.example.id
}
