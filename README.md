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

## Authentication

You will have to set the `FLOWS_TOKEN` environment variable for authentication. You can obtain it in two ways.

### API Key

Go to your avatar in the left upper corner => Settings => Authorizations => API Keys and create a new personal API key. Make sure to give it editor access to any relevant projects, along with `api` and `flows:edit` capabilities.

You can then use it for authentication:

```shell
export FLOWS_TOKEN=sfapi_...
```

### flowctl

For local development, you can also get a token from an authenticated `flowctl` CLI.

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
