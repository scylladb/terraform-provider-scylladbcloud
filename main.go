package main

import (
	"context"
	"flag"
	"log"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/provider"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5/tf5server"
)

// Generate the Terraform provider documentation using `tfplugindocs`:
//go:generate go tool tfplugindocs generate --rendered-provider-name "ScyllaDB Cloud" --examples-dir ./examples --website-source-dir ./templates

func main() {
	debugFlag := flag.Bool("debug", false, "Start provider in debug mode.")
	flag.Parse()

	serverFactory, _, err := provider.ProtoV5ProviderServerFactory(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	var serveOpts []tf5server.ServeOpt

	if *debugFlag {
		serveOpts = append(serveOpts, tf5server.WithManagedDebug())
	}

	logFlags := log.Flags()
	logFlags = logFlags &^ (log.Ldate | log.Ltime)
	log.SetFlags(logFlags)

	err = tf5server.Serve(
		"registry.terraform.io/scylladb/scylladbcloud",
		serverFactory,
		serveOpts...,
	)
	if err != nil {
		log.Fatal(err)
	}
}
