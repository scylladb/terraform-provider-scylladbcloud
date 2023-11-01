terraform-provider-scylladbcloud
================================


### Getting started with local development and testing

Let's assume we have a $GOBIN env set to the following path:

```
$ export GOBIN=/Users/rafal/bin
```

Create a local development override file, to teach Terraform to use your $GOBIN
when looking up for provider binary, instead of talking with Terraform Registry:

```
$ cat >~/.terraformrc <<EOF
provider_installation {
	dev_overrides {
		"registry.terraform.io/scylladb/scylladbcloud" = "/Users/rafal/bin"
	}
}
EOF
```

Build the provider binary and move it to $GOBIN:

```
$ go install github.com/scylladb/terraform-provider-scylladbcloud
```

Take one of the example templates and configure the provider section with proper
values for the `token` and `endpoint` keys.

Run `terraform plan` or `terraform apply` and be happy!

For debugging / troubleshooting please [Terraform debugging documentation](https://developer.hashicorp.com/terraform/internals/debugging).


### Demo Video

[![Watch a demo of using Scylla Cloud Terrafrom Provider](https://img.youtube.com/vi/ccsACgHHDYo/hqdefault.jpg)](https://www.youtube.com/embed/ccsACgHHDYo)
