package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-mux/tf5muxserver"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// ProtoV5ProviderServerFactory returns a muxed terraform-plugin-go protocol v5 provider factory function.
// This factory function is suitable for use with the terraform-plugin-go Serve function.
// The primary (Plugin SDK) provider server is also returned (useful for testing).
func ProtoV5ProviderServerFactory(ctx context.Context) (func() tfprotov5.ProviderServer, *schema.Provider, error) {
	primary, err := New(ctx)
	if err != nil {
		return nil, nil, err
	}

	servers := []func() tfprotov5.ProviderServer{
		primary.GRPCProvider,
	}

	muxServer, err := tf5muxserver.NewMuxServer(ctx, servers...)
	if err != nil {
		return nil, nil, err
	}

	return muxServer.ProviderServer, primary, nil
}
