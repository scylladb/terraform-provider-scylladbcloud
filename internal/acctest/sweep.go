package acctest

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
)

// SweepClusters deletes each cluster whose name starts with Prefix and that
// is not deleted yet. It writes one line with logf for each cluster it
// deletes or cannot delete. A failed delete does not stop the sweep: the
// errors are returned together at the end. It never touches a cluster
// without the prefix.
func SweepClusters(ctx context.Context, c *scylla.Client, logf func(format string, args ...any)) error {
	clusters, err := c.ListClusters(ctx)
	if err != nil {
		return fmt.Errorf("failed to list clusters: %w", err)
	}

	var errs []error
	deleted := 0
	for _, cl := range clusters {
		if !strings.HasPrefix(cl.ClusterName, Prefix) || isDeletedStatus(cl.Status) {
			continue
		}
		if _, err := c.DeleteCluster(ctx, cl.ID, cl.ClusterName, 0); err != nil {
			logf("[ERROR] sweeper: cannot delete cluster %d %q: %s", cl.ID, cl.ClusterName, err)
			errs = append(errs, fmt.Errorf("cluster %d %q: %w", cl.ID, cl.ClusterName, err))
			continue
		}
		logf("[INFO] sweeper: deleted cluster %d %q (status %s)", cl.ID, cl.ClusterName, cl.Status)
		deleted++
	}
	logf("[INFO] sweeper: deleted %d clusters, %d failed", deleted, len(errs))
	return errors.Join(errs...)
}

func isDeletedStatus(status string) bool {
	return strings.EqualFold(status, "DELETED") || strings.EqualFold(status, "DELETING")
}
