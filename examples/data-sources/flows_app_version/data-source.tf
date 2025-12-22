data "flows_app_version" "example" {
  registry = "Registry Name"
  name     = "My App Name"
  version  = "1.0.0"
  custom   = false
}

resource "flows_app_installation" "example" {
  project_id = "your-project-id"
  name       = "My Custom Installation"

  app = {
    version_id = data.flows_app_version.example.id
    custom     = data.flows_app_version.example.custom
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
