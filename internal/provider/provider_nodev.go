//go:build !dev

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func devSchema(map[string]*schema.Schema) {}

func devConfigure(_ context.Context, _ *schema.ResourceData, _, token string) (devSettings, diag.Diagnostics) {
	return devSettings{token: token}, nil
}
