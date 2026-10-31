package serverless

import (
	"strings"
	"testing"
)

func TestDeprecationNamesRemovalVersion(t *testing.T) {
	cluster, bundle := ResourceServerlessCluster(), DataSourceServerlessBundle()
	for name, msgs := range map[string][]string{
		"scylladbcloud_serverless_cluster": {cluster.DeprecationMessage, cluster.Description},
		"scylladbcloud_serverless_bundle":  {bundle.DeprecationMessage, bundle.Description},
	} {
		for _, msg := range msgs {
			if !strings.Contains(msg, "removed in v2.0.0") || !strings.Contains(msg, "scylladbcloud_cluster") {
				t.Errorf("%s: %q does not name the removal version and the replacement", name, msg)
			}
		}
	}
}
