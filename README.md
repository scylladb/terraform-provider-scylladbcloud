terraform-provider-scylladbcloud
================================

This is the repository for the Terraform Scylla Cloud Provider, which allows one to use Terraform with ScyllaDB's Database as a Service, Scylla Cloud. For general information about Terraform, visit the official website and the GitHub project page. For details about Scylla Cloud, see [Scylla Cloud Documentation](https://cloud.docs.scylladb.com).
The provider is using [Scylla Cloud REST API](https://cloud.docs.scylladb.com/stable/api-docs/api-get-started.html).


### Prerequisites

* Terrafrom 0.13+
* Go 1.18 (to build the provider plugin)
* [Scylla Cloud](https://cloud.scylladb.com/) Account
* [Scylla Cloud API Token](https://cloud.docs.scylladb.com/stable/api-docs/api-get-started.html#obtaining-an-api-key-beta)

### Provider configuration

In order to configure provider pass a token you obtained from ScyllaDB Cloud:

```
terraform {
	required_providers {
		scylladbcloud = {
			source = "registry.terraform.io/scylladb/scylladbcloud"
		}
	}
}

provider "scylladbcloud" {
	token = "..."
}
```

Run `terraform apply` in order to create a cluster or `terraform destroy` in order to delete it.

You can also import an existing cluster by providing its ID:

```
resource "scylladbcloud_cluster" "mycluster" { }
```

Run `terraform import scylladbcloud_cluster.mycluster 123` to import an existing cluster into the state file.

For debugging / troubleshooting please [Terraform debugging documentation](https://developer.hashicorp.com/terraform/internals/debugging).


### Demo Video

[![Watch a demo of using Scylla Cloud Terrafrom Provider](https://img.youtube.com/vi/ccsACgHHDYo/hqdefault.jpg)](https://www.youtube.com/embed/ccsACgHHDYo)
