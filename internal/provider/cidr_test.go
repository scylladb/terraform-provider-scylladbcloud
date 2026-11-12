package provider

import (
	"fmt"
	"regexp"
	"strings"
)

// testAccCIDRIndex gives every acceptance test its own cluster CIDR block, so
// tests running in parallel against one account never overlap, which VPC
// peering and transit gateway attachments would reject. Keys are the name
// purposes passed to acctest.RandomWithPrefix after testAccPrefix.
var testAccCIDRIndex = map[string]int{
	"allowlist":                    1,
	"backup-retention-default":     2,
	"basic-aws":                    3,
	"basic-aws-migration-v1-to-v2": 4,
	"basic-aws-scale-out":          5,
	"basic-aws-scale-out-outside":  6,
	"basic-aws-single-az":          7,
	"basic-gcp":                    8,
	"basic-gcp-byoa":               9,
	"connection":                   10,
	"cql-auth-v0":                  11,
	"ear-cmk":                      12,
	"ear-default":                  13,
	"ear-default-gcp":              14,
	"ear-disabled":                 15,
	"ear-enabled":                  16,
	"ear-gcp":                      17,
	"ear-no-diff-on-upgrade":       18,
	"vpc-peering":                  19,
	"vpc-peering-v0":               20,
	"xcloud-aws":                   21,
}

var testAccRandomSuffix = regexp.MustCompile(`-[0-9]+$`)

// testAccCIDR returns the /24 cluster CIDR block of the test that created a
// resource named name.
func testAccCIDR(name string) string {
	purpose := testAccRandomSuffix.ReplaceAllString(strings.TrimPrefix(name, testAccPrefix), "")
	i, ok := testAccCIDRIndex[purpose]
	if !ok {
		panic(fmt.Sprintf("no CIDR block assigned to acceptance test %q; add it to testAccCIDRIndex", purpose))
	}
	return fmt.Sprintf("10.0.%d.0/24", i)
}
