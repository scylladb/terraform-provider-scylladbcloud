package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
)

func markAttrsAsComputed(attrs map[string]tfsdk.Attribute) map[string]tfsdk.Attribute {
	for k, v := range attrs {
		v.Computed = true
		attrs[k] = v
	}
	return attrs
}

func extractAttrsTypes(attrs map[string]tfsdk.Attribute) map[string]attr.Type {
	attrsTypes := make(map[string]attr.Type, len(attrs))
	for k, v := range attrs {
		attrsTypes[k] = v.Type
	}
	return attrsTypes
}
