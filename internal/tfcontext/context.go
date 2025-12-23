package tfcontext

import (
	"context"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const endpointContextKey = "endpoint"

const (
	methodContextKey  = "method"
	apiPathContextKey = "path"
)

func AddProviderInfo(ctx context.Context, endpoint string) context.Context {
	return tflog.SetField(ctx, endpointContextKey, endpoint)
}

func AddHttpRequestInfo(ctx context.Context, method, path string) context.Context {
	ctx = tflog.SetField(ctx, methodContextKey, method)
	return tflog.SetField(ctx, apiPathContextKey, path)
}
