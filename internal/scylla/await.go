package scylla

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
)

// DefaultPollInterval is the PollInterval that NewClient sets.
const DefaultPollInterval = 10 * time.Second

// WaitClusterRequestEnd returns the cluster request once it has ended, with
// status COMPLETED or FAILED. It returns an error only when it cannot get the
// end status: an API error, a timeout or an unknown status. It checks the
// request at once, then every PollInterval.
func (c *Client) WaitClusterRequestEnd(ctx context.Context, requestID int64) (model.ClusterRequest, error) {
	t := time.NewTicker(c.PollInterval)
	defer t.Stop()

	for {
		r, err := c.GetClusterRequest(ctx, requestID)
		if err != nil {
			return r, fmt.Errorf("failed to get cluster request with ID %d: %w", requestID, err)
		}

		switch {
		case r.Completed(), r.Failed():
			return r, nil
		case strings.EqualFold(r.Status, "QUEUED"), strings.EqualFold(r.Status, "IN_PROGRESS"):
		default:
			return r, fmt.Errorf("cluster request ID %d has unknown status %q", r.ID, r.Status)
		}

		select {
		case <-t.C:
		case <-ctx.Done():
			return r, fmt.Errorf("cluster request ID %d is still %s (%s at %d%%): %w",
				r.ID, r.Status, r.RequestType, r.ProgressPercent, ctx.Err())
		}
	}
}

// AwaitClusterRequest returns once the cluster request has completed, or an
// error once it has failed.
func (c *Client) AwaitClusterRequest(ctx context.Context, requestID int64) error {
	r, err := c.WaitClusterRequestEnd(ctx, requestID)
	if err != nil {
		return err
	}
	if r.Failed() {
		return fmt.Errorf("cluster request ID %d failed (%s at %d%%): %s",
			r.ID, r.RequestType, r.ProgressPercent, r.ProgressDescription)
	}
	return nil
}

// AwaitNoInProgressRequests returns once the cluster has no request in
// progress. It checks the cluster at once, then every PollInterval.
func (c *Client) AwaitNoInProgressRequests(ctx context.Context, clusterID int64) error {
	t := time.NewTicker(c.PollInterval)
	defer t.Stop()

	for {
		reqs, err := c.ListClusterRequest(ctx, clusterID, ListClusterRequestParams{Status: "IN_PROGRESS"})
		if err != nil || len(reqs) == 0 {
			return err
		}

		select {
		case <-t.C:
		case <-ctx.Done():
			r := reqs[0]
			return fmt.Errorf("cluster ID %d still has request ID %d in progress (%s at %d%%): %w",
				clusterID, r.ID, r.RequestType, r.ProgressPercent, ctx.Err())
		}
	}
}
