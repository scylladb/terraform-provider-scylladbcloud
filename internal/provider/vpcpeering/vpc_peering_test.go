package vpcpeering

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/eapache/go-resiliency/retrier"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
)

const (
	testCloudAWS = 1
	testCloudGCP = 2
)

// newTestClient returns a client of the fake API at srv. AWS deploys clusters
// only in us-east-1 and GCP only in us-central1.
func newTestClient(t *testing.T, srv *httptest.Server) *scylla.Client {
	t.Helper()

	endpoint, err := url.Parse(srv.URL)
	require.NoError(t, err)

	return &scylla.Client{
		Endpoint:   endpoint,
		Headers:    make(http.Header),
		HTTPClient: srv.Client(),
		Retry:      retrier.New(nil, nil),
		AccountID:  7,
		Meta: &scylla.Cloudmeta{
			CloudProviders: []scylla.CloudProvider{
				{
					CloudProvider: &model.CloudProvider{ID: testCloudAWS, Name: "AWS"},
					CloudProviderRegions: &model.CloudProviderRegions{
						Regions: []model.CloudProviderRegion{{ID: 1, ExternalID: "us-east-1"}},
					},
				},
				{
					CloudProvider: &model.CloudProvider{ID: testCloudGCP, Name: "GCP"},
					CloudProviderRegions: &model.CloudProviderRegions{
						Regions: []model.CloudProviderRegion{{ID: 40, ExternalID: "us-central1"}},
					},
				},
			},
			GCPBlocks: map[string]string{
				"us-central1":  "10.128.0.0/20",
				"europe-west1": "10.132.0.0/20",
			},
		},
	}
}

func TestResourceVPCPeeringCreate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		cloud  int
		region string
		cidrs  []any
		// wantSent holds the region and CIDR fields of the create request,
		// a missing key means the field was not sent.
		wantSent map[string]any
		wantErr  string
	}{
		{
			name:     "AWS deployment region sends its name and ID",
			cloud:    testCloudAWS,
			region:   "us-east-1",
			cidrs:    []any{"192.168.0.0/16"},
			wantSent: map[string]any{"region": "us-east-1", "regionId": float64(1), "cidrBlock": "192.168.0.0/16"},
		},
		{
			name:     "AWS deployment region in uppercase",
			cloud:    testCloudAWS,
			region:   "US-EAST-1",
			cidrs:    []any{"192.168.0.0/16"},
			wantSent: map[string]any{"region": "us-east-1", "regionId": float64(1), "cidrBlock": "192.168.0.0/16"},
		},
		{
			name:     "AWS region without clusters sends its name only",
			cloud:    testCloudAWS,
			region:   "eu-central-2",
			cidrs:    []any{"192.168.0.0/16"},
			wantSent: map[string]any{"region": "eu-central-2", "cidrBlock": "192.168.0.0/16"},
		},
		{
			name:     "AWS region without clusters in uppercase",
			cloud:    testCloudAWS,
			region:   "EU-CENTRAL-2",
			cidrs:    []any{"192.168.0.0/16"},
			wantSent: map[string]any{"region": "eu-central-2", "cidrBlock": "192.168.0.0/16"},
		},
		{
			name:    "AWS requires CIDR blocks",
			cloud:   testCloudAWS,
			region:  "us-east-1",
			wantErr: `"peer_cidr_blocks" is required for "AWS" cloud`,
		},
		{
			name:     "GCP sends no region",
			cloud:    testCloudGCP,
			region:   "us-central1",
			cidrs:    []any{"10.10.0.0/16"},
			wantSent: map[string]any{"cidrBlock": "10.10.0.0/16"},
		},
		{
			name:     "GCP region without a default CIDR block takes explicit CIDR blocks",
			cloud:    testCloudGCP,
			region:   "me-central2",
			cidrs:    []any{"10.10.0.0/16"},
			wantSent: map[string]any{"cidrBlock": "10.10.0.0/16"},
		},
		{
			// The provider used to reject a list that starts with the default.
			name:     "GCP CIDR blocks may start with the default one",
			cloud:    testCloudGCP,
			region:   "us-central1",
			cidrs:    []any{"10.128.0.0/20", "10.10.0.0/16"},
			wantSent: map[string]any{"cidrBlock": "10.128.0.0/20,10.10.0.0/16"},
		},
		{
			name:     "GCP default CIDR block of a region without clusters",
			cloud:    testCloudGCP,
			region:   "europe-west1",
			wantSent: map[string]any{"cidrBlock": "10.132.0.0/20"},
		},
		{
			name:     "GCP default CIDR block of a region in uppercase",
			cloud:    testCloudGCP,
			region:   "US-CENTRAL1",
			wantSent: map[string]any{"cidrBlock": "10.128.0.0/20"},
		},
		{
			name:    "GCP region without a default CIDR block needs explicit CIDR blocks",
			cloud:   testCloudGCP,
			region:  "me-central2",
			wantErr: `no default peer CIDR block for "me-central2" region, set "peer_cidr_blocks"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var sent map[string]any

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Method + " " + r.URL.Path {
				case "GET /account/7/cluster/42/dcs":
					_, _ = fmt.Fprintf(w, `{"data":{"dataCenters":[{"id":5,"Name":"DC1","CloudProviderID":%d}]}}`, tt.cloud)
				case "POST /account/7/cluster/42/network/vpc/peer":
					if err := json.NewDecoder(r.Body).Decode(&sent); err != nil {
						t.Errorf("decode create request: %v", err)
					}
					_, _ = w.Write([]byte(`{"data":{"id":9,"externalId":"pcx-1"}}`))
				case "GET /account/7/cluster/42/network/vpc/peer/9":
					_, _ = w.Write([]byte(`{"data":{"id":9,"externalId":"pcx-1"}}`))
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
			}))
			defer srv.Close()

			raw := map[string]any{
				"cluster_id":      42,
				"datacenter":      "DC1",
				"peer_vpc_id":     "vpc-1",
				"peer_region":     tt.region,
				"peer_account_id": "123",
				"allow_cql":       true,
			}
			if tt.cidrs != nil {
				raw["peer_cidr_blocks"] = tt.cidrs
			}
			d := schema.TestResourceDataRaw(t, ResourceVPCPeering().Schema, raw)

			diags := resourceVPCPeeringCreate(context.Background(), d, newTestClient(t, srv))

			if tt.wantErr != "" {
				require.True(t, diags.HasError())
				require.Equal(t, tt.wantErr, diags[0].Summary)
				require.Nil(t, sent, "nothing should be created")
				return
			}

			require.False(t, diags.HasError(), "diags: %v", diags)
			require.Equal(t, "pcx-1", d.Id())

			got := make(map[string]any)
			for _, key := range []string{"region", "regionId", "cidrBlock"} {
				if v, ok := sent[key]; ok {
					got[key] = v
				}
			}
			require.Equal(t, tt.wantSent, got)
		})
	}
}

func TestResourceVPCPeeringRead(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		cloud int
		// regionFields are the region fields of the peering in the API
		// response.
		regionFields string
		// prevRegion is peer_region before the read, empty for an import.
		prevRegion string
		wantRegion string
	}{
		{
			// The ID is of us-east-1, so the case fails if the ID wins.
			name:         "region name wins over the region ID",
			cloud:        testCloudAWS,
			regionFields: `"region":"eu-central-2","regionId":1`,
			prevRegion:   "EU-CENTRAL-2",
			wantRegion:   "eu-central-2",
		},
		{
			name:         "import of an AWS region without clusters",
			cloud:        testCloudAWS,
			regionFields: `"region":"eu-central-2"`,
			wantRegion:   "eu-central-2",
		},
		{
			name:         "API that returns the region ID only",
			cloud:        testCloudAWS,
			regionFields: `"regionId":1`,
			wantRegion:   "us-east-1",
		},
		{
			name:       "GCP keeps the configured region",
			cloud:      testCloudGCP,
			prevRegion: "europe-west1",
			wantRegion: "europe-west1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			peering := `{"id":9,"externalId":"pcx-1","vpcId":"vpc-1","cidrList":["192.168.0.0/16"]`
			if tt.regionFields != "" {
				peering += "," + tt.regionFields
			}
			peering += "}"

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Method + " " + r.URL.Path {
				case "GET /account/7/clusters":
					_, _ = w.Write([]byte(`{"data":{"clusters":[{"id":42}]}}`))
				case "GET /account/7/cluster/42":
					_, _ = fmt.Fprintf(w, `{"data":{"cluster":{"id":42,"cloudProviderId":%d,"dc":{"id":5,"Name":"DC1"},`+
						`"dataCenters":[{"id":5,"Name":"DC1"}],"vpcPeeringList":[%s]}}}`, tt.cloud, peering)
				case "GET /account/7/cluster/42/dc/5":
					_, _ = w.Write([]byte(`{"data":{"id":5,"Name":"DC1"}}`))
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
			}))
			defer srv.Close()

			raw := map[string]any{}
			if tt.prevRegion != "" {
				raw["peer_region"] = tt.prevRegion
			}
			d := schema.TestResourceDataRaw(t, ResourceVPCPeering().Schema, raw)
			d.SetId("pcx-1")

			diags := resourceVPCPeeringRead(context.Background(), d, newTestClient(t, srv))

			require.False(t, diags.HasError(), "diags: %v", diags)
			require.Equal(t, "pcx-1", d.Id())
			require.Equal(t, tt.wantRegion, d.Get("peer_region"))
		})
	}
}

// testState returns the state of a peering in region with cidrs.
func testState(region string, cidrs ...string) *terraform.InstanceState {
	attrs := map[string]string{
		"id":                 "pcx-1",
		"cluster_id":         "42",
		"datacenter":         "DC1",
		"peer_vpc_id":        "vpc-1",
		"peer_cidr_blocks.#": strconv.Itoa(len(cidrs)),
		"peer_region":        region,
		"peer_account_id":    "123",
		"allow_cql":          "true",
		"vpc_peering_id":     "9",
		"connection_id":      "pcx-1",
		"network_link":       "projects//global/networks/",
	}
	for i, cidr := range cidrs {
		attrs["peer_cidr_blocks."+strconv.Itoa(i)] = cidr
	}

	return &terraform.InstanceState{ID: "pcx-1", Attributes: attrs}
}

// testConfig returns the configuration of a peering in region. With nil
// cidrs, the configuration does not set peer_cidr_blocks.
func testConfig(region string, cidrs []any) *terraform.ResourceConfig {
	raw := map[string]any{
		"cluster_id":      42,
		"datacenter":      "DC1",
		"peer_vpc_id":     "vpc-1",
		"peer_region":     region,
		"peer_account_id": "123",
		"allow_cql":       true,
	}
	if cidrs != nil {
		raw["peer_cidr_blocks"] = cidrs
	}

	return terraform.NewResourceConfigRaw(raw)
}

func TestResourceVPCPeeringPeerRegionDiff(t *testing.T) {
	t.Parallel()

	const cidr = "192.168.0.0/16"

	tests := []struct {
		name         string
		stateRegion  string
		configRegion string
		wantReplace  bool
	}{
		{name: "same region", stateRegion: "us-east-1", configRegion: "us-east-1"},
		{name: "same region in uppercase", stateRegion: "us-east-1", configRegion: "US-EAST-1"},
		{name: "region without clusters in uppercase", stateRegion: "eu-central-2", configRegion: "EU-CENTRAL-2"},
		{name: "other region", stateRegion: "us-east-1", configRegion: "eu-central-2", wantReplace: true},
		{name: "other region in uppercase", stateRegion: "us-east-1", configRegion: "EU-CENTRAL-2", wantReplace: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			state := testState(tt.stateRegion, cidr)
			config := testConfig(tt.configRegion, []any{cidr})

			diff, err := ResourceVPCPeering().Diff(context.Background(), state, config, nil)
			require.NoError(t, err)

			if !tt.wantReplace {
				require.True(t, diff.Empty(), "unexpected diff: %v", diff)
				return
			}

			require.NotNil(t, diff)
			require.True(t, diff.RequiresNew())
			require.Contains(t, diff.Attributes, "peer_region")
		})
	}
}
