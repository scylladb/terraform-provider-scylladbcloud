package acctest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
)

// SweepClusters deletes each cluster whose name starts with Prefix and that
// is not deleted yet. It writes one line with logf for each resource it
// deletes or cannot delete. It never touches a cluster without the prefix.
//
// It first deletes the VPC peerings and the connections of each cluster and
// waits until they are gone. A cluster delete does not delete its peerings,
// and after the delete they cannot be listed. So when this cleanup fails, it
// keeps the cluster, and the next sweep can try again. Then it sends all
// cluster deletes and waits for each delete request.
//
// A failed step does not stop the sweep: the errors are returned together at
// the end.
func SweepClusters(ctx context.Context, c *scylla.Client, logf func(format string, args ...any)) error {
	clusters, err := c.ListClusters(ctx)
	if err != nil {
		return fmt.Errorf("failed to list clusters: %w", err)
	}

	type pending struct {
		cluster model.Cluster
		request int64
	}
	var (
		errs    []error
		deletes []pending
	)
	fail := func(cl model.Cluster, err error) {
		logf("[ERROR] sweeper: cluster %d %q: %s", cl.ID, cl.ClusterName, err)
		errs = append(errs, fmt.Errorf("cluster %d %q: %w", cl.ID, cl.ClusterName, err))
	}

	for _, cl := range clusters {
		if !strings.HasPrefix(cl.ClusterName, Prefix) || isDeletedStatus(cl.Status) {
			continue
		}
		if err := sweepNetwork(ctx, c, cl, logf); err != nil {
			fail(cl, fmt.Errorf("%w; the cluster is kept, run the sweep again", err))
			continue
		}
		r, err := c.DeleteCluster(ctx, cl.ID, cl.ClusterName, 0)
		if err != nil {
			fail(cl, fmt.Errorf("cannot delete the cluster: %w", err))
			continue
		}
		deletes = append(deletes, pending{cluster: cl, request: r.ID})
	}

	deleted := 0
	for _, p := range deletes {
		if err := c.AwaitClusterRequest(ctx, p.request); err != nil {
			fail(p.cluster, fmt.Errorf("the delete did not complete: %w", err))
			continue
		}
		logf("[INFO] sweeper: deleted cluster %d %q (status %s)", p.cluster.ID, p.cluster.ClusterName, p.cluster.Status)
		deleted++
	}
	logf("[INFO] sweeper: deleted %d clusters, %d failed", deleted, len(errs))
	return errors.Join(errs...)
}

// sweepNetwork deletes the VPC peerings and the connections of a cluster and
// waits until they are gone. Only a private cluster has them: the API
// rejects these calls for other clusters.
func sweepNetwork(ctx context.Context, c *scylla.Client, cl model.Cluster, logf func(format string, args ...any)) error {
	if cl.BroadcastType != "PRIVATE" {
		return nil
	}

	peerings, err := c.ListClusterVPCPeerings(ctx, cl.ID)
	if err != nil {
		return fmt.Errorf("cannot list VPC peerings: %w", err)
	}
	conns, err := c.ListClusterConnections(ctx, cl.ID)
	if err != nil {
		return fmt.Errorf("cannot list connections: %w", err)
	}

	var errs []error
	for _, p := range peerings {
		if err := c.DeleteClusterVPCPeering(ctx, cl.ID, p.ID); err != nil {
			errs = append(errs, fmt.Errorf("cannot delete VPC peering %d: %w", p.ID, err))
			continue
		}
		logf("[INFO] sweeper: deleting VPC peering %d (%s) of cluster %d", p.ID, p.ExternalID, cl.ID)
	}
	var deleting []int64
	for _, conn := range conns {
		if err := c.DeleteClusterConnection(ctx, cl.ID, conn.ID); err != nil {
			errs = append(errs, fmt.Errorf("cannot delete connection %d: %w", conn.ID, err))
			continue
		}
		logf("[INFO] sweeper: deleting connection %d of cluster %d", conn.ID, cl.ID)
		deleting = append(deleting, conn.ID)
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	// A peering delete is a cluster request. Wait for all open requests of
	// the cluster, so that the cluster delete does not run before them.
	if len(peerings) > 0 {
		if err := awaitOpenRequests(ctx, c, cl.ID); err != nil {
			return fmt.Errorf("VPC peering delete did not complete: %w", err)
		}
	}
	for _, id := range deleting {
		if err := awaitConnectionDeleted(ctx, c, cl.ID, id); err != nil {
			return err
		}
	}
	return nil
}

func awaitOpenRequests(ctx context.Context, c *scylla.Client, clusterID int64) error {
	for _, status := range []string{"QUEUED", "IN_PROGRESS"} {
		reqs, err := c.ListClusterRequest(ctx, clusterID, scylla.ListClusterRequestParams{Status: status})
		if err != nil {
			return err
		}
		for _, r := range reqs {
			if err := c.AwaitClusterRequest(ctx, r.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func awaitConnectionDeleted(ctx context.Context, c *scylla.Client, clusterID, connID int64) error {
	t := time.NewTicker(c.PollInterval)
	defer t.Stop()
	for {
		conn, err := c.GetClusterConnection(ctx, clusterID, connID)
		switch {
		case scylla.IsNotFound(err), scylla.IsClusterConnectionDeletedErr(err):
			return nil
		case err != nil:
			return fmt.Errorf("cannot read connection %d: %w", connID, err)
		case strings.EqualFold(conn.Status, "DELETED"):
			return nil
		}
		select {
		case <-t.C:
		case <-ctx.Done():
			return fmt.Errorf("connection %d is still %s: %w", connID, conn.Status, ctx.Err())
		}
	}
}

func isDeletedStatus(status string) bool {
	return strings.EqualFold(status, "DELETED") || strings.EqualFold(status, "DELETING")
}
