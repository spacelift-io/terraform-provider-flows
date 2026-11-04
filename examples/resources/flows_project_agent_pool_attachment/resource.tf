# Attach a private agent pool to a project
resource "flows_project_agent_pool_attachment" "example" {
  project_id    = "your-project-id"
  agent_pool_id = "your-private-agent-pool-id"
}

# Attach the same private agent pool to several projects
locals {
  project_ids = [
    "first-project-id",
    "second-project-id",
  ]
}

resource "flows_project_agent_pool_attachment" "many" {
  for_each = toset(local.project_ids)

  project_id    = each.value
  agent_pool_id = "your-private-agent-pool-id"
}
