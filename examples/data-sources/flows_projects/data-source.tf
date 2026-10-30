# All projects the token has access to.
data "flows_projects" "all" {}

# Only users' personal sandbox projects.
data "flows_projects" "sandboxes" {
  personal_sandbox {
    equals = true
  }
}

# Projects with one of the given names.
data "flows_projects" "selected" {
  name {
    any_of = ["Platform", "Security"]
  }
}

# The `ids` attribute is a set, so it can be used directly with for_each.
resource "flows_secret" "sandbox_api_key" {
  for_each = data.flows_projects.sandboxes.ids

  project_id = each.value
  key        = "API_KEY"
  value      = var.api_key
}

output "sandbox_project_names" {
  value = data.flows_projects.sandboxes.projects[*].name
}
