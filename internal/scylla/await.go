package scylla

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
)

// DefaultPollInterval is how often cluster requests are polled when
// Client.PollInterval is unset.
const DefaultPollInterval = 10 * time.Second

func (c *Client) pollInterval() time.Duration {
	if c.PollInterval > 0 {
		return c.PollInterval
	}
	return DefaultPollInterval
}

// AwaitClusterRequest returns once the cluster request has completed, or an
// error once it has failed.
func (c *Client) AwaitClusterRequest(ctx context.Context, requestID int64) error {
	t := time.NewTicker(c.pollInterval())
	defer t.Stop()

	for {
		select {
		case <-t.C:
			r, err := c.GetClusterRequest(ctx, requestID)
			if err != nil {
				return fmt.Errorf("failed to get cluster request with ID %d: %w", requestID, err)
			}

			switch {
			case strings.EqualFold(r.Status, "COMPLETED"):
				return nil
			case strings.EqualFold(r.Status, "FAILED"):
				return clusterRequestFailed(r)
			case strings.EqualFold(r.Status, "QUEUED"), strings.EqualFold(r.Status, "IN_PROGRESS"):
				continue
			}

			return fmt.Errorf("unknown cluster request status: %s", r.Status)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// genericFailure is the progress description of a failure the API gives no
// reason for.
const genericFailure = "Registered error during request processing"

// clusterRequestFailed describes a failed request with what the API says
// about it. The public API exposes only the progress description, not the
// internal error behind the failure.
func clusterRequestFailed(r model.ClusterRequest) error {
	msg := fmt.Sprintf("cluster request ID %d failed (%s at %d%%)", r.ID, r.RequestType, r.ProgressPercent)
	if d := strings.TrimSpace(r.ProgressDescription); d != "" && d != genericFailure {
		msg += ": " + d
	}
	return errors.New(msg)
}

// AwaitNoInProgressRequests returns once the cluster has no request in
// progress.
func (c *Client) AwaitNoInProgressRequests(ctx context.Context, clusterID int64) error {
	t := time.NewTicker(c.pollInterval())
	defer t.Stop()

	allClear := func() (bool, error) {
		reqs, err := c.ListClusterRequest(ctx, clusterID, ListClusterRequestParams{Status: "IN_PROGRESS"})
		if err != nil {
			return false, err
		}
		return len(reqs) == 0, nil
	}

	for {
		ok, err := allClear()
		if err != nil || ok {
			return err
		}

		select {
		case <-t.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
