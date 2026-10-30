package provider

import (
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	acc "github.com/scylladb/terraform-provider-scylladbcloud/internal/acctest"
)

// testAccErrorRegexp matches msg in Terraform's output, which wraps long
// diagnostics across lines.
func testAccErrorRegexp(msg string) *regexp.Regexp {
	words := strings.Fields(msg)
	for i, w := range words {
		words[i] = regexp.QuoteMeta(w)
	}
	return regexp.MustCompile(strings.Join(words, `\s+`))
}

// TestAccScyllaDBCloudCluster_invalidConfig checks that invalid cluster
// configurations fail at plan time with the message users see. Nothing is
// created.
func TestAccScyllaDBCloudCluster_invalidConfig(t *testing.T) {
	fixed := config.Variables{
		"node_type": config.StringVariable("i3.large"),
		"min_nodes": config.IntegerVariable(3),
	}
	scaling := func(v config.Variables) config.Variables {
		v["scaling"] = config.BoolVariable(true)
		return v
	}
	families := config.ListVariable(config.StringVariable("i4i"))
	types := config.ListVariable(config.StringVariable("i4i.large"))

	cases := []struct {
		name string
		vars config.Variables
		err  string
	}{
		{
			name: "min_nodes too low",
			vars: config.Variables{"node_type": config.StringVariable("i3.large"), "min_nodes": config.IntegerVariable(2)},
			err:  "min_nodes must be at least 3, got 2",
		},
		{
			name: "min_nodes not divisible by 3",
			vars: config.Variables{"node_type": config.StringVariable("i3.large"), "min_nodes": config.IntegerVariable(4)},
			err:  "min_nodes must be divisible by 3, got 4",
		},
		{
			name: "no size",
			vars: config.Variables{},
			err:  `either configure the "scaling" block or set both "min_nodes" and "node_type"`,
		},
		{
			name: "node_type without min_nodes",
			vars: config.Variables{"node_type": config.StringVariable("i3.large")},
			err:  `"min_nodes" is required when the "scaling" block is not configured`,
		},
		{
			name: "min_nodes without node_type",
			vars: config.Variables{"min_nodes": config.IntegerVariable(3)},
			err:  `"node_type" is required when the "scaling" block is not configured`,
		},
		{
			name: "scaling without instances",
			vars: scaling(config.Variables{}),
			err:  `exactly one of "instance_families" or "instance_types" must be configured in the "scaling" block`,
		},
		{
			name: "scaling with families and types",
			vars: scaling(config.Variables{"instance_families": families, "instance_types": types}),
			err:  `exactly one of "instance_families" or "instance_types" must be configured in the "scaling" block`,
		},
		{
			name: "scaling with min_nodes",
			vars: scaling(config.Variables{"instance_families": families, "min_nodes": config.IntegerVariable(3)}),
			err:  `"min_nodes": conflicts with scaling`,
		},
		{
			name: "scaling with node_type",
			vars: scaling(config.Variables{"instance_families": families, "node_type": config.StringVariable("i3.large")}),
			err:  `"node_type": conflicts with scaling`,
		},
		{
			name: "target_utilization too high",
			vars: scaling(config.Variables{"instance_families": families, "target_utilization": config.FloatVariable(0.95)}),
			err:  "target_utilization must be greater than 0.0 and less than or equal to 0.9, got 0.95",
		},
		{
			name: "target_utilization zero",
			vars: scaling(config.Variables{"instance_families": families, "target_utilization": config.FloatVariable(0.0)}),
			err:  "target_utilization must be greater than 0.0 and less than or equal to 0.9, got 0",
		},
		{
			name: "backup_retention_days too high",
			vars: withVars(fixed, config.Variables{"backup_retention_days": config.IntegerVariable(61)}),
			err:  "backup_retention_days must be between 0 and 60, got 61",
		},
		{
			name: "key_id with encryption disabled",
			vars: withVars(fixed, config.Variables{
				"encryption_at_rest":         config.BoolVariable(true),
				"encryption_at_rest_enabled": config.BoolVariable(false),
				"encryption_at_rest_key_id":  config.StringVariable("key-deadbeef"),
			}),
			err: `"key_id" cannot be set when "enabled" is false in the "encryption_at_rest" block`,
		},
		{
			name: "malformed key_id",
			vars: withVars(fixed, config.Variables{
				"encryption_at_rest":         config.BoolVariable(true),
				"encryption_at_rest_enabled": config.BoolVariable(true),
				"encryption_at_rest_key_id":  config.StringVariable("arn:aws:kms:key"),
			}),
			err: `invalid "key_id" "arn:aws:kms:key" in the "encryption_at_rest" block`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resource.ParallelTest(t, resource.TestCase{
				PreCheck:                 func() { acc.PreCheck(t) },
				ProtoV5ProviderFactories: protoV5ProviderFactories,
				Steps: []resource.TestStep{
					{
						ConfigDirectory: config.StaticDirectory("testdata/cluster_invalid"),
						ConfigVariables: tc.vars,
						PlanOnly:        true,
						ExpectError:     testAccErrorRegexp(tc.err),
					},
				},
			})
		})
	}
}

func withVars(base config.Variables, extra config.Variables) config.Variables {
	v := config.Variables{}
	for k, val := range base {
		v[k] = val
	}
	for k, val := range extra {
		v[k] = val
	}
	return v
}
