# Terraform Provider for Spacelift Flows

Terraform provider for [Spacelift Flows](https://spacelift.io/flows).

## Using the Provider

The provider is published to the Terraform and OpenTofu registries. Add it to your TF configuration:

```hcl
terraform {
  required_providers {
    flows = {
      source = "spacelift-io/flows"
    }
  }
}

provider "flows" {
  endpoint = "https://useflows.eu"  # or https://useflows.us
}
```

Set the `FLOWS_TOKEN` environment variable for authentication:

```shell
export FLOWS_TOKEN=$(flowctl auth token)
```

## Usage

The provider supports managing flows as code and entity lifecycle confirmations.

### Example: Creating a Flow

```hcl
resource "flows_flow" "example" {
  project_id = "your-project-id"
  name       = "my-flow"
  definition = file("${path.module}/flow.yaml")

  app_installation_mapping = {
    my_app = "app-installation-id"
  }
}

resource "flows_entity_lifecycle_confirmation" "example" {
  entity_id = flows_flow.example.blocks["my_entity"].id
}
```

See the [examples](./examples/) directory for more usage examples.
