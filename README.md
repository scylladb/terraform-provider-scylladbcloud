terraform-provider-scylladbcloud
================================

This is the repository for the Terraform Scylla Cloud Provider, which allows one to use Terraform with ScyllaDB's Database-as-a-Service, ScyllaDB Cloud. For general information about Terraform, visit the official website and the GitHub project page. For details about ScyllaDB Cloud, see [ScyllaDB Cloud Documentation](https://cloud.docs.scylladb.com).
The provider is using [ScyllaDB Cloud REST API](https://cloud.docs.scylladb.com/stable/api-docs/api-get-started.html).


### Prerequisites

* Terrafrom 0.13+
* Go 1.18 (to build the provider plugin)
* [ScyllaDB Cloud](https://cloud.scylladb.com/) Account
* [ScyllaDB Cloud API Token](https://cloud.docs.scylladb.com/stable/api-docs/create-api-token.html)

### Provider configuration

In order to configure the provider, pass a token you obtained from ScyllaDB Cloud:

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

### Example Usage

ScyllaDB Cloud supports two cluster types. The type is determined by configuration you provide:

- *Standard clusters* – use `node_type` and `min_nodes` to define the instance and minimum node count. The cluster scales out automatically when storage thresholds are reached, but instance type and minimum node count remain under your control.
- *X Cloud clusters* – use the `scaling` block instead of `node_type` and `min_nodes`. The control plane handles autoscaling automatically based on the policy you define. `node_type` and `min_nodes` must not be set when scaling is present.

**Standard cluster**
```terraform
resource "scylladbcloud_cluster" "standard" {
  name       = "My_Standard_Cluster"
  cloud      = "AWS"
  region     = "us-east-1"
  node_type  = "i3.large"
  min_nodes  = 3
  cidr_block = "172.31.0.0/16"
}
```

**X Cloud cluster**
```terraform
resource "scylladbcloud_cluster" "xcloud" {
  name       = "My_XCloud_Cluster"
  cloud      = "AWS"
  region     = "us-east-1"
  cidr_block = "172.31.0.0/16"

  scaling {
    instance_families = ["i8g"]
    storage_policy {
      min_gb             = 500
      target_utilization = 0.75
    }
    vcpu_policy {
      min = 6
    }
  }
}
```

You can also import an existing cluster by providing its ID:

```
resource "scylladbcloud_cluster" "mycluster" { }
```

Run `terraform import scylladbcloud_cluster.mycluster 123` to import an existing cluster into the state file.

For debugging / troubleshooting please see [Terraform debugging documentation](https://developer.hashicorp.com/terraform/internals/debugging).


### Demo Video

[![Watch a demo of using Scylla Cloud Terrafrom Provider](https://img.youtube.com/vi/ccsACgHHDYo/hqdefault.jpg)](https://www.youtube.com/embed/ccsACgHHDYo)
