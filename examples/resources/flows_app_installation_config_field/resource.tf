resource "flows_app_installation" "example" {
  project_id = "your-project-id"
  name       = "My App Installation"

  app = {
    version_id = "app-version-id"
    custom     = true
  }
}

resource "flows_app_installation_config_field" "api_key" {
  app_installation_id = flows_app_installation.example.id
  key                 = "api_key"
  value               = "secret-api-key-value"
}

resource "flows_app_installation_config_field" "webhook_url" {
  app_installation_id = flows_app_installation.example.id
  key                 = "webhook_url"
  value               = "https://example.com/webhook"
}
