package acctest

import (
	sdkacctest "github.com/hashicorp/terraform-plugin-testing/helper/acctest"
)

// Prefix starts the name of each resource that the acceptance tests create.
// The sweepers delete only resources with this prefix.
const Prefix = "tf-acc-test-"

// RandomName returns a new name for a test resource: Prefix, the purpose of
// the test and a random suffix, for example "tf-acc-test-basic-aws-x62svfkq".
func RandomName(purpose string) string {
	return Prefix + purpose + "-" + sdkacctest.RandString(8)
}
