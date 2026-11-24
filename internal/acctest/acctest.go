// Package acctest holds helpers for the acceptance tests, which run against
// the real ScyllaDB Cloud API.
package acctest

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"slices"
	"strconv"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
	v2scylla "github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/v2"
)

const (
	envEndpoint = "SCYLLADB_CLOUD_ENDPOINT"
	envToken    = "SCYLLADB_CLOUD_TOKEN"
	envTrace    = "SCYLLADB_CLOUD_TRACE"
)

// NewClient returns a client for the API in SCYLLADB_CLOUD_ENDPOINT, with the
// token in SCYLLADB_CLOUD_TOKEN. Like the provider, it sends the trace in
// SCYLLADB_CLOUD_TRACE, or a new trace when that variable is not set. It
// needs no *testing.T, so code outside a test, such as a sweeper, can use it.
func NewClient() (*scylla.Client, error) {
	endpoint, token := os.Getenv(envEndpoint), os.Getenv(envToken)
	if endpoint == "" || token == "" {
		return nil, fmt.Errorf("%s and %s must be set for acceptance tests", envEndpoint, envToken)
	}
	trace, err := v2scylla.TraceOrNew(os.Getenv(envTrace))
	if err != nil {
		return nil, fmt.Errorf("could not generate trace id: %w", err)
	}
	c, err := scylla.NewClient(endpoint, token, scylla.UserAgent(""), trace, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create the acceptance test client: %w", err)
	}
	return c, nil
}

// sharedClient builds the client once per test binary, because the client
// loads the deployment metadata from the API.
var sharedClient = sync.OnceValues(NewClient)

// PreCheck fails the test when the client cannot be created, for example
// because a variable is not set or the token is not valid. A test must find
// this before it creates resources.
func PreCheck(t testing.TB) {
	t.Helper()
	Client(t)
}

// Client returns the shared client. It fails the test when the client cannot
// be created. Do not call it in a check function: the framework calls
// CheckDestroy in a deferred function, where t.Fatal hides the error.
func Client(t testing.TB) *scylla.Client {
	t.Helper()
	c, err := sharedClient()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// GCPBYOAID returns the GCP BYOA credential ID for the BYOA tests, or "" when
// it is not set.
func GCPBYOAID() string {
	return os.Getenv("TEST_SCYLLADB_CLOUD_GCP_BYOA_ID")
}

// EARKeyID returns the customer-managed key for the encryption at rest tests,
// or "" when it is not set.
func EARKeyID() string {
	return os.Getenv("TEST_SCYLLADB_CLOUD_EAR_KEY_ID")
}

// clusterID returns the cluster ID of a scylladbcloud_cluster in the state.
func clusterID(rs *terraform.ResourceState) (int64, error) {
	if rs.Primary == nil || rs.Primary.ID == "" {
		return 0, fmt.Errorf("cluster ID not set")
	}
	id, err := strconv.ParseInt(rs.Primary.ID, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid cluster ID %q: %w", rs.Primary.ID, err)
	}
	return id, nil
}

// CheckClusterExists returns a check that reads the cluster of resourceName
// from the API into cluster.
func CheckClusterExists(t testing.TB, resourceName string, cluster *model.Cluster) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %q not found", resourceName)
		}

		id, err := clusterID(rs)
		if err != nil {
			return err
		}

		c, err := sharedClient()
		if err != nil {
			return err
		}
		got, err := c.GetCluster(t.Context(), id)
		if err != nil {
			return fmt.Errorf("error retrieving cluster %d: %w", id, err)
		}

		*cluster = *got
		return nil
	}
}

// CheckClusterDestroyed returns a check that fails unless the API confirms
// that each cluster in the state is deleted.
func CheckClusterDestroyed(t testing.TB) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c, err := sharedClient()
		if err != nil {
			return err
		}
		return checkClusterDestroyed(t.Context(), c, s)
	}
}

func checkClusterDestroyed(ctx context.Context, c *scylla.Client, s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "scylladbcloud_cluster" {
			continue
		}

		id, err := clusterID(rs)
		if err != nil {
			return err
		}

		if err := awaitClusterDeleted(ctx, c, id); err != nil {
			return err
		}
	}

	return nil
}

// awaitClusterDeleted returns once the API confirms that the cluster is
// deleted. Delete does not wait until the cluster is gone (CLOUD-5783), so
// the cluster can still exist when the check starts. It then waits for the
// delete request to complete.
func awaitClusterDeleted(ctx context.Context, c *scylla.Client, id int64) error {
	reqs, err := c.ListClusterRequest(ctx, id, scylla.ListClusterRequestParams{Type: "DELETE_CLUSTER"})
	switch {
	case isGone(err):
		return nil
	case err != nil:
		// Any other error, such as a 5xx or an expired token, does not
		// show that the cluster is gone. A leaked cluster costs money.
		return fmt.Errorf("cannot confirm that cluster %d is destroyed: %w", id, err)
	case len(reqs) == 0:
		return fmt.Errorf("cluster %d still exists, no delete request was sent", id)
	}

	last := slices.MaxFunc(reqs, func(a, b model.ClusterRequest) int { return cmp.Compare(a.ID, b.ID) })
	if err := c.AwaitClusterRequest(ctx, last.ID); err != nil {
		return fmt.Errorf("cannot confirm that cluster %d is destroyed: %w", id, err)
	}
	return nil
}

// isGone reports whether err is an API answer that the object does not exist.
func isGone(err error) bool {
	return scylla.IsNotFound(err) || scylla.IsClusterDeletedErr(err) || scylla.IsDeletedErr(err)
}
