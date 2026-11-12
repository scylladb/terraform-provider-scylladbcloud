package cqlauth

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/scyllatest"
)

// v0State decodes state the way the SDK hands it to a state upgrader: plain
// JSON, so numbers are float64.
func v0State(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &m))
	return m
}

func TestUpgradeFromV0(t *testing.T) {
	t.Parallel()
	for name, tt := range map[string]struct {
		datacenterID int
		want         string
	}{
		"no datacenter":       {datacenterID: 0, want: "AWS_US_EAST_1"},
		"datacenter by ID":    {datacenterID: 6, want: "aws_eu_west_1"},
		"datacenter mismatch": {datacenterID: 9},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			srv := scyllatest.New(t)
			srv.Reply(http.MethodGet, "/account/1/cluster/connect", map[string]any{
				"connectDataCenters": []any{
					map[string]any{"dcName": "AWS_US_EAST_1"},
					map[string]any{"dcName": "aws_eu_west_1"},
				},
			})
			srv.Reply(http.MethodGet, "/account/1/cluster/3/dcs", map[string]any{"dataCenters": []any{
				map[string]any{"id": 5, "name": "AWS_US_EAST_1"},
				map[string]any{"id": 6, "name": "AWS_EU_WEST_1"},
			}})
			raw := v0State(t, `{"id":"seeds","cluster_id":3,"datacenter_id":`+jsonInt(tt.datacenterID)+
				`,"dns":true,"seeds":"10.0.0.1","username":"scylla","password":"secret"}`)

			got, err := dataSourceCQLAuthUpgradeFrom0(context.Background(), raw, srv.Client(t))
			if tt.want == "" {
				require.ErrorContains(t, err, "not found")
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got["datacenter"])
			require.Equal(t, 3.0, got["cluster_id"])
			require.Equal(t, "secret", got["password"])
			require.NotContains(t, got, "datacenter_id")
		})
	}
}

func jsonInt(i int) string {
	b, _ := json.Marshal(i)
	return string(b)
}
