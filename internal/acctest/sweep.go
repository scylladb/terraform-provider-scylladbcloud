package acctest

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
)

// SweepClusters deletes each cluster whose name starts with Prefix and that
// is not deleted yet. It writes one line with logf for each resource it
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
		errs = append(errs, sweepNetwork(ctx, c, cl, logf)...)
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

// sweepNetwork deletes the VPC peerings and the connections of a cluster. A
// cluster delete does not delete its VPC peerings, and after the delete they
// cannot be listed, so they must go first. Connections go first too.
func sweepNetwork(ctx context.Context, c *scylla.Client, cl model.Cluster, logf func(format string, args ...any)) []error {
	var errs []error
	fail := func(what string, err error) {
		logf("[ERROR] sweeper: cluster %d %q: %s: %s", cl.ID, cl.ClusterName, what, err)
		errs = append(errs, fmt.Errorf("cluster %d %q: %s: %w", cl.ID, cl.ClusterName, what, err))
	}

	peerings, err := c.ListClusterVPCPeerings(ctx, cl.ID)
	if err != nil {
		fail("cannot list VPC peerings", err)
	}
	for _, p := range peerings {
		if err := c.DeleteClusterVPCPeering(ctx, cl.ID, p.ID); err != nil {
			fail(fmt.Sprintf("cannot delete VPC peering %d", p.ID), err)
			continue
		}
		logf("[INFO] sweeper: deleted VPC peering %d (%s) of cluster %d", p.ID, p.ExternalID, cl.ID)
	}

	conns, err := c.ListClusterConnections(ctx, cl.ID)
	if err != nil {
		fail("cannot list connections", err)
	}
	for _, conn := range conns {
		if err := c.DeleteClusterConnection(ctx, cl.ID, conn.ID); err != nil {
			fail(fmt.Sprintf("cannot delete connection %d", conn.ID), err)
			continue
		}
		logf("[INFO] sweeper: deleted connection %d of cluster %d", conn.ID, cl.ID)
	}
	return errs
}

func isDeletedStatus(status string) bool {
	return strings.EqualFold(status, "DELETED") || strings.EqualFold(status, "DELETING")
}
