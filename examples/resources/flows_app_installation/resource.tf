resource "flows_app_installation" "example" {
  project_id     = "your-project-id"
  name           = "My Custom Installation"
  app_version_id = "app-installation-version-id"

  config_fields = {
    example: "\"example-value\""
  }

  style_override = {
    color: "#ff0000"
  }

  custom_registry  = true
  confirm          = true
  wait_for_confirm = true
}
