package provider

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/acctest"
)

// TestMain runs the sweepers instead of the tests when go test gets -sweep.
// See make sweep.
func TestMain(m *testing.M) {
	resource.TestMain(m)
}

func init() {
	// The API lists the clusters of all regions in one call, so F ignores
	// the -sweep region.
	resource.AddTestSweepers("scylladbcloud_cluster", &resource.Sweeper{
		Name: "scylladbcloud_cluster",
		F: func(string) error {
			c, err := acctest.NewClient()
			if err != nil {
				return err
			}
			// End before the -timeout of make sweep, so the sweep reports
			// what it could not finish.
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
			defer cancel()
			return acctest.SweepClusters(ctx, c, log.Printf)
		},
	})
}
