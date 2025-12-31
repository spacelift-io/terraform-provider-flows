resource "flows_app_installation" "example" {
  project_id = "your-project-id"
  name       = "My App Installation"

  app = {
    version_id = "app-version-id"
    custom     = true
  }

  config_fields = {
    database_url = "\"postgresql://localhost/mydb\""
  }

  confirm        = true
  wait_for_ready = false
}

resource "flows_app_installation_wait_for_ready" "example" {
  app_installation_id = flows_app_installation.example.id
}
