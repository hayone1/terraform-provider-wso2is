package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/hayone1/terraform-provider-wso2is/internal/provider"
)

// Set by goreleaser.
var version = "dev"

//go:generate go tool tfplugindocs generate --provider-name wso2is

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "start the provider with support for debuggers like delve")
	flag.Parse()

	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/hayone1/wso2is",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err.Error())
	}
}
