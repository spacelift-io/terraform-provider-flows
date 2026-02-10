resource "flows_data_table" "example" {
  project_id = "your-project-id"
  name       = "my-data-table"
}

# String column
resource "flows_data_table_column" "name_column" {
  data_table_id = flows_data_table.example.id
  name          = "name"
  type          = "string"
}

# Integer column
resource "flows_data_table_column" "age_column" {
  data_table_id = flows_data_table.example.id
  name          = "age"
  type          = "integer"
}

# Float column
resource "flows_data_table_column" "float_column" {
  data_table_id = flows_data_table.example.id
  name          = "price"
  type          = "float"
}

# Datetime column
resource "flows_data_table_column" "date_column" {
  data_table_id = flows_data_table.example.id
  name          = "date"
  type          = "datetime"
}

# Boolean column
resource "flows_data_table_column" "active_column" {
  data_table_id = flows_data_table.example.id
  name          = "active"
  type          = "boolean"
}

# JSON column
resource "flows_data_table_column" "metadata_column" {
  data_table_id = flows_data_table.example.id
  name          = "metadata"
  type          = "json"
}

# Create a second table for reference testing
resource "flows_data_table" "users_table" {
  project_id = "your-project-id"
  name       = "users"
}

# Reference column (row_ref type)
resource "flows_data_table_column" "user_ref_column" {
  data_table_id = flows_data_table.example.id
  name          = "assigned_user"
  type          = "row_ref"
  ref_table_id  = flows_data_table.users_table.id
}
