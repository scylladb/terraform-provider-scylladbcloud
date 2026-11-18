package scylla

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// DefaultPollInterval is the PollInterval that NewClient sets.
const DefaultPollInterval = 10 * time.Second

// AwaitClusterRequest returns once the cluster request has completed, or an
// error once it has failed. It checks the request at once, then every
// PollInterval.
func (c *Client) AwaitClusterRequest(ctx context.Context, requestID int64) error {
	t := time.NewTicker(c.PollInterval)
	defer t.Stop()

	for {
		r, err := c.GetClusterRequest(ctx, requestID)
		if err != nil {
			return fmt.Errorf("failed to get cluster request with ID %d: %w", requestID, err)
		}

		switch {
		case strings.EqualFold(r.Status, "COMPLETED"):
			return nil
		case strings.EqualFold(r.Status, "FAILED"):
			return fmt.Errorf("cluster request ID %d failed", r.ID)
		case strings.EqualFold(r.Status, "QUEUED"), strings.EqualFold(r.Status, "IN_PROGRESS"):
		default:
			return fmt.Errorf("unknown cluster request status: %s", r.Status)
		}

		select {
		case <-t.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
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
			return ctx.Err()
		}
	}
}
