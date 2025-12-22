resource "flows_app_installation" "example" {
  project_id = "your-project-id"
  name       = "My Custom Installation"

  app = {
    version_id = "app-installation-version-id"
    custom     = true
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
