resource "flows_app_installation" "example" {
  project_id = "your-project-id"
  name       = "My App Installation"

  app = {
    version_id = "app-version-id"
    custom     = true
  }

  confirm        = false
  wait_for_ready = false
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

resource "flows_app_installation_confirmation" "example" {
  app_installation_id = flows_app_installation.example.id
  wait_for_ready      = true

  depends_on = [
    flows_app_installation_config_field.api_key,
    flows_app_installation_config_field.webhook_url,
  ]
}
