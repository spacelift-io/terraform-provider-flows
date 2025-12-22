datasource "flows_app_version" "example" {
  registry    = "Registry Name"
  app_name    = "My App Name"
  app_version = "1.0.0"
  custom      = "false"
}

resource "flows_app_installation" "example" {
  project_id = "your-project-id"
  name       = "My Custom Installation"

  app = {
    version_id = datsource.flows_app_version.example.id
    custom     = datasource.flows_app_version.example.custom
  }

  config_fields = {
    example: "\"example-value\""
  }

  style_override = {
    color: "#ff0000"
  }

  confirm        = true
  wait_for_ready = true
}
