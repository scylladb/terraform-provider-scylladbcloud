---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "scylladbcloud Provider"
subcategory: ""
description: |-

---

# ScyllaDB Cloud Provider

This provider allows you to manage [ScyllaDB Cloud](https://cloud.scylladb.com/) resources using Terraform.
You must configure the provider with proper credentials before you can use it. See
[Obtaining an API Key](https://cloud.docs.scylladb.com/stable/api-docs/api-get-started.html#obtaining-an-api-key-beta) for instructions on getting an API access token.

Use the navigation menu on the left to read about the available data sources and resources.


## Example Usage

```terraform
terraform {
	required_providers {
		scylladbcloud = {
			source = "registry.terraform.io/scylladb/scylladbcloud"
		}
	}
}

# Configuration-based authentication.
provider "scylladbcloud" {
	token = "..." # Replace with bearer token obtained from ScyllaDB Cloud
}
```

### Environment Variables

Authentication token can be provided by using the `SCYLLADB_CLOUD_TOKEN` environment variable.

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `token` (String, Sensitive) Bearer token used to authenticate with the API. If not provided, the `SCYLLADB_CLOUD_TOKEN` environment variable is used.

### Optional

- `endpoint` (String) URL of the Scylla Cloud endpoint.

## Useful Links

[Reporting bugs or feature requests](https://github.com/scylladb/terraform-provider-scylladbcloud/issues)

[ScyllaDB Cloud documentation](https://cloud.docs.scylladb.com/)

[ScyllaDB Cloud support](https://cloud.scylladb.com/support)

[ScyllaDB community forum](https://forum.scylladb.com/)
