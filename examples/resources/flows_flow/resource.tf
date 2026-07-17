resource "flows_flow" "example" {
  project_id = "your-project-id"
  name       = "My Flow"
  definition = file("${path.module}/flow.yaml")
  enabled    = true

  app_installation_mapping = {
    my_app = "app-installation-id"
  }
}
