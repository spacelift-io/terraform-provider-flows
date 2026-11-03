# Look a project up by name instead of hard-coding its ID.
data "flows_project" "sandbox" {
  name = "My Personal Sandbox"
}

resource "flows_flow" "example" {
  project_id = data.flows_project.sandbox.id
  name       = "example"
  definition = file("${path.module}/flow.yaml")
}

# Projects can also be looked up by ID to retrieve their metadata.
data "flows_project" "by_id" {
  id = "019c5689-e85c-72e9-9cc8-322de9274ae9"
}
