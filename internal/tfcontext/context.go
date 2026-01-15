package tfcontext

import (
	"context"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	methodContextKey  = "method"
	apiPathContextKey = "path"
)

func AddHttpRequestInfo(ctx context.Context, method, path string) context.Context {
	ctx = tflog.SetField(ctx, methodContextKey, method)
	return tflog.SetField(ctx, apiPathContextKey, path)
}
