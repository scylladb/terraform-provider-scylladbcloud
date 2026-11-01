// Package acctest builds the ScyllaDB Cloud API client that acceptance tests
// and sweepers use to inspect and clean up resources, independently of any
// provider instance.
package acctest

import (
	"context"
	"errors"
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
)

const userAgent = "terraform-provider-scylladbcloud/acctest"

func envEndpoint() string { return os.Getenv("SCYLLADB_CLOUD_ENDPOINT") }

func envToken() string { return os.Getenv("SCYLLADB_CLOUD_TOKEN") }

func envDevMode() bool {
	v, _ := strconv.ParseBool(os.Getenv("SCYLLADB_CLOUD_DEV_MODE"))
	return v
}

// The client loads deployment metadata on creation, so it is built once per
// process and shared.
var shared = sync.OnceValues(func() (*scylla.Client, error) {
	return newClient(context.Background())
})

// NewClient returns the shared client configured from the environment. It
// needs no *testing.T, so sweepers can use it.
func NewClient() (*scylla.Client, error) {
	return shared()
}

// Client returns the shared client, failing the test when it cannot be built.
func Client(t testing.TB) *scylla.Client {
	t.Helper()
	c, err := NewClient()
	if err != nil {
		t.Fatalf("acceptance test API client: %s", err)
	}
	return c
}

// PreCheck fails the test when the environment cannot run acceptance tests.
func PreCheck(t testing.TB) {
	t.Helper()
	if err := checkEnv(); err != nil {
		t.Fatal(err)
	}
}

func checkEnv() error {
	if envEndpoint() == "" {
		return errors.New("SCYLLADB_CLOUD_ENDPOINT must be set for acceptance tests")
	}
	if envToken() == "" && (!devBuild || !envDevMode()) {
		return errors.New("SCYLLADB_CLOUD_TOKEN must be set for acceptance tests")
	}
	return nil
}

func newClient(ctx context.Context) (*scylla.Client, error) {
	if err := checkEnv(); err != nil {
		return nil, err
	}
	endpoint, token := envEndpoint(), envToken()
	var opts []scylla.ClientOption
	if devBuild && envDevMode() {
		var err error
		if token, opts, err = devLogin(ctx, endpoint, token); err != nil {
			return nil, err
		}
	}
	return scylla.NewClient(endpoint, token, userAgent, "", true, opts...)
}
