package provider

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	acc "github.com/scylladb/terraform-provider-scylladbcloud/internal/acctest"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
)

// testAccPrefix starts the name of everything acceptance tests create, so the
// sweeper can find what a killed run left behind without touching anything
// else.
const testAccPrefix = "tf-acc-test-"

func TestMain(m *testing.M) {
	resource.TestMain(m)
}

func init() {
	resource.AddTestSweepers("scylladbcloud_cluster", &resource.Sweeper{
		Name: "scylladbcloud_cluster",
		F: func(string) error {
			c, err := acc.NewClient()
			if err != nil {
				return err
			}
			return sweepClusters(context.Background(), c)
		},
	})
}

// sweepClusters deletes every live cluster whose name has testAccPrefix. A
// failed delete is logged and the sweep goes on; the errors are returned
// together at the end.
func sweepClusters(ctx context.Context, c *scylla.Client) error {
	clusters, err := c.ListClusters(ctx)
	if err != nil {
		return fmt.Errorf("listing clusters: %w", err)
	}
	var errs []error
	for _, cl := range clusters {
		if !strings.HasPrefix(cl.ClusterName, testAccPrefix) || strings.EqualFold(cl.Status, "DELETED") {
			continue
		}
		log.Printf("[INFO] sweeping cluster %d %q", cl.ID, cl.ClusterName)
		if _, err := c.DeleteCluster(ctx, cl.ID, cl.ClusterName, 0); err != nil {
			log.Printf("[ERROR] deleting cluster %d %q: %s", cl.ID, cl.ClusterName, err)
			errs = append(errs, fmt.Errorf("cluster %d %q: %w", cl.ID, cl.ClusterName, err))
		}
	}
	return errors.Join(errs...)
}
