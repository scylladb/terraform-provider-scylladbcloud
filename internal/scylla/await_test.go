package scylla_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/scyllatest"
)

const requestPath = "/account/1/cluster/request/7"

func request(status string) scyllatest.Response {
	return scyllatest.Response{Data: model.ClusterRequest{ID: 7, Status: status}}
}

func failed(requestType string, percent int64, description string) scyllatest.Response {
	return scyllatest.Response{Data: model.ClusterRequest{
		ID: 7, Status: "FAILED", RequestType: requestType, ProgressPercent: percent, ProgressDescription: description,
	}}
}

func TestAwaitClusterRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		responses []scyllatest.Response
		wantErr   string
		exact     bool
		wantPolls int
	}{
		{name: "completed after progress", responses: []scyllatest.Response{request("QUEUED"), request("IN_PROGRESS"), request("COMPLETED")}, wantPolls: 3},
		{name: "status is case insensitive", responses: []scyllatest.Response{request("completed")}, wantPolls: 1},
		{name: "failed", responses: []scyllatest.Response{request("IN_PROGRESS"), request("FAILED")}, wantErr: "cluster request ID 7 failed", wantPolls: 2},
		{name: "failed with description", responses: []scyllatest.Response{failed("CREATE_CLUSTER", 30, "Registered error during request processing. Not enough capacity")}, wantErr: "cluster request ID 7 failed (CREATE_CLUSTER at 30%): Registered error during request processing. Not enough capacity", exact: true, wantPolls: 1},
		{name: "failed without description", responses: []scyllatest.Response{failed("RESIZE_CLUSTER", 0, "")}, wantErr: "cluster request ID 7 failed (RESIZE_CLUSTER at 0%): ", exact: true, wantPolls: 1},
		{name: "unknown status", responses: []scyllatest.Response{request("PAUSED")}, wantErr: `cluster request ID 7 has unknown status "PAUSED"`, exact: true, wantPolls: 1},
		{name: "api error", responses: []scyllatest.Response{{Status: http.StatusOK, Error: "040001"}}, wantErr: "failed to get cluster request with ID 7", wantPolls: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := scyllatest.New(t)
			srv.Sequence(http.MethodGet, requestPath, tt.responses...)

			err := srv.Client(t).AwaitClusterRequest(context.Background(), 7)
			switch {
			case tt.wantErr == "":
				require.NoError(t, err)
			case tt.exact:
				require.EqualError(t, err, tt.wantErr)
			default:
				require.ErrorContains(t, err, tt.wantErr)
			}
			require.Len(t, srv.CallsTo(http.MethodGet, requestPath), tt.wantPolls)
		})
	}
}

func TestAwaitClusterRequestChecksBeforeWaiting(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Sequence(http.MethodGet, requestPath, request("COMPLETED"))
	c := srv.Client(t)
	c.PollInterval = time.Hour

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, c.AwaitClusterRequest(ctx, 7))
}

func TestAwaitClusterRequestTimeout(t *testing.T) {
	t.Parallel()
	srv := scyllatest.New(t)
	srv.Sequence(http.MethodGet, requestPath, scyllatest.Response{Data: model.ClusterRequest{
		ID: 7, Status: "IN_PROGRESS", RequestType: "CREATE_CLUSTER", ProgressPercent: 85,
	}})
	// The interval is longer than the deadline, so the deadline always ends
	// the wait between two checks.
	c := srv.Client(t)
	c.PollInterval = time.Hour

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	err := c.AwaitClusterRequest(ctx, 7)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.EqualError(t, err, "cluster request ID 7 is still IN_PROGRESS (CREATE_CLUSTER at 85%): context deadline exceeded")
}

func TestAwaitNoInProgressRequests(t *testing.T) {
	t.Parallel()
	const path = "/account/1/cluster/3/request"
	busy := scyllatest.Response{Data: []model.ClusterRequest{{ID: 9, Status: "IN_PROGRESS"}}}
	idle := scyllatest.Response{Data: []model.ClusterRequest{}}

	t.Run("idle returns without waiting", func(t *testing.T) {
		t.Parallel()
		srv := scyllatest.New(t)
		srv.Sequence(http.MethodGet, path, idle)
		c := srv.Client(t)
		c.PollInterval = time.Hour

		require.NoError(t, c.AwaitNoInProgressRequests(context.Background(), 3))
		calls := srv.CallsTo(http.MethodGet, path)
		require.Len(t, calls, 1)
		require.Equal(t, "status=IN_PROGRESS", calls[0].Query)
	})

	t.Run("waits until idle", func(t *testing.T) {
		t.Parallel()
		srv := scyllatest.New(t)
		srv.Sequence(http.MethodGet, path, busy, busy, idle)

		require.NoError(t, srv.Client(t).AwaitNoInProgressRequests(context.Background(), 3))
		require.Len(t, srv.CallsTo(http.MethodGet, path), 3)
	})

	t.Run("timeout names the request in progress", func(t *testing.T) {
		t.Parallel()
		srv := scyllatest.New(t)
		srv.Sequence(http.MethodGet, path, scyllatest.Response{Data: []model.ClusterRequest{
			{ID: 9, Status: "IN_PROGRESS", RequestType: "RESIZE_CLUSTER", ProgressPercent: 50},
		}})
		c := srv.Client(t)
		c.PollInterval = time.Hour

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
		defer cancel()
		err := c.AwaitNoInProgressRequests(ctx, 3)
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.EqualError(t, err, "cluster ID 3 still has request ID 9 in progress (RESIZE_CLUSTER at 50%): context deadline exceeded")
	})
}

func TestDefaultPollInterval(t *testing.T) {
	t.Parallel()
	require.Equal(t, 10*time.Second, scylla.DefaultPollInterval)
}
