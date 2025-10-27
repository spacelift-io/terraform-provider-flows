resource "flows_flow" "example" {
  project_id = "your-project-id"
  name       = "My Flow"
  definition = file("${path.module}/flow.yaml")

  app_installation_mapping = {
    my_app = "app-installation-id"
  }
}

resource "flows_entity_lifecycle_confirmation" "example" {
  entity_id = flows_flow.example.blocks["my_entity"].id
}
