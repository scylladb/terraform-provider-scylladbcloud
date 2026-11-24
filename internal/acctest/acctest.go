// Package acctest holds helpers for the acceptance tests, which run against
// the real ScyllaDB Cloud API.
package acctest

import (
	"fmt"
	"os"
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

var (
	sharedClientMu sync.Mutex
	sharedClientC  *scylla.Client
)

// sharedClient builds the client once per test binary, because the client
// loads the deployment metadata from the API. It keeps only a client built
// without an error, so one short API error does not fail every later test.
func sharedClient() (*scylla.Client, error) {
	sharedClientMu.Lock()
	defer sharedClientMu.Unlock()
	if sharedClientC != nil {
		return sharedClientC, nil
	}
	c, err := NewClient()
	if err != nil {
		return nil, err
	}
	sharedClientC = c
	return c, nil
}

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

// CheckClusterDestroyed returns a check that fails when a cluster in the
// state still exists.
func CheckClusterDestroyed(t testing.TB) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c, err := sharedClient()
		if err != nil {
			return err
		}

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "scylladbcloud_cluster" {
				continue
			}

			id, err := clusterID(rs)
			if err != nil {
				return err
			}

			if _, err := c.GetCluster(t.Context(), id); err == nil {
				return fmt.Errorf("cluster %d still exists", id)
			}
		}

		return nil
	}
}
