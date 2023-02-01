terraform-provider-scylladbcloud
================================

This is the repository for the Terraform Scylla Cloud Provider, which allows one to use Terraform with ScyllaDB's Database as a Service, Scylla Cloud. For general information about Terraform, visit the official website and the GitHub project page. For details about Scylla Cloud, see [Scylla Cloud Documentation](https://cloud.docs.scylladb.com).
The provider is using [Scylla Cloud REST API](https://cloud.docs.scylladb.com/stable/api-docs/api-get-started.html).


### Prerequisites

* Terrafrom 0.13+
* Go 1.18 (to build the provider plugin)
* [Scylla Cloud](https://cloud.scylladb.com/) Account
* [Scylla Cloud API Token](https://cloud.docs.scylladb.com/stable/api-docs/api-get-started.html#obtaining-an-api-key-beta)

### Getting started with local development and testing

Let's assume we have a $GOBIN env set to the following path:

```
$ export GOBIN=${HOME}/bin
```

Create a local development override file, to teach Terraform to use your $GOBIN
when looking up for provider binary, instead of talking with Terraform Registry:

```
$ cat >~/.terraformrc <<EOF
provider_installation {
	dev_overrides {
		"registry.terraform.io/scylladb/scylladbcloud" = "${HOME}/bin"
	}
}
EOF
```

Build the provider binary and move it to $GOBIN:

```
$ go install github.com/scylladb/terraform-provider-scylladbcloud
```

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
