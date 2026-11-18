package vpcpeering

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/eapache/go-resiliency/retrier"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/go-cty/cty/msgpack"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
)

const (
	testCloudAWS = 1
	testCloudGCP = 2

	testResourceType = "scylladbcloud_vpc_peering"

	// testAPICIDR is the CIDR block of the peering in the fake API.
	testAPICIDR = "192.168.0.0/16"
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
			wantErr: `"peer_cidr_blocks" is required: "me-central2" has no default CIDR block`,
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

			// The state has the CIDR blocks that were sent, as a list. A GCP
			// default is one CIDR block.
			wantCIDRs := tt.cidrs
			if wantCIDRs == nil {
				wantCIDRs = []any{tt.wantSent["cidrBlock"]}
			}
			require.Equal(t, wantCIDRs, d.Get("peer_cidr_blocks"))
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

			srv := newReadAPI(t, tt.cloud, tt.regionFields)

			raw := map[string]any{}
			if tt.prevRegion != "" {
				// The CIDR blocks are not the ones of the API, so the read
				// must replace them.
				raw["peer_region"] = tt.prevRegion
				raw["peer_cidr_blocks"] = []any{"10.0.0.0/8"}
			}
			d := schema.TestResourceDataRaw(t, ResourceVPCPeering().Schema, raw)
			d.SetId("pcx-1")

			diags := resourceVPCPeeringRead(context.Background(), d, newTestClient(t, srv))

			require.False(t, diags.HasError(), "diags: %v", diags)
			require.Equal(t, "pcx-1", d.Id())
			require.Equal(t, tt.wantRegion, d.Get("peer_region"))
			require.Equal(t, []any{testAPICIDR}, d.Get("peer_cidr_blocks"))
		})
	}
}

// newReadAPI returns a fake API with cluster 42 of cloud and its peering
// pcx-1, with regionFields in the peering.
func newReadAPI(t *testing.T, cloud int, regionFields string) *httptest.Server {
	t.Helper()

	peering := fmt.Sprintf(`{"id":9,"externalId":"pcx-1","vpcId":"vpc-1","cidrList":[%q]`, testAPICIDR)
	if regionFields != "" {
		peering += "," + regionFields
	}
	peering += "}"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET /account/7/clusters":
			_, _ = w.Write([]byte(`{"data":{"clusters":[{"id":42,"dcCount":1}]}}`))
		case "GET /account/7/cluster/42":
			_, _ = fmt.Fprintf(w, `{"data":{"cluster":{"id":42,"cloudProviderId":%d,"dc":{"id":5,"Name":"DC1"},`+
				`"dataCenters":[{"id":5,"Name":"DC1"}],"vpcPeeringList":[%s]}}}`, cloud, peering)
		case "GET /account/7/cluster/42/dc/5":
			_, _ = w.Write([]byte(`{"data":{"id":5,"Name":"DC1"}}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)

	return srv
}

// testProviderServer returns the provider server that Terraform talks to,
// with the peering resource only.
func testProviderServer(client *scylla.Client) tfprotov5.ProviderServer {
	p := &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{testResourceType: ResourceVPCPeering()},
	}
	p.SetMeta(client)

	return schema.NewGRPCProviderServer(p)
}

// newNoAPI returns a fake API that fails the test on any request.
func newNoAPI(t *testing.T) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
	}))
	t.Cleanup(srv.Close)

	return srv
}

func peeringType() cty.Type {
	return ResourceVPCPeering().CoreConfigSchema().ImpliedType()
}

// testPeering returns a peering object with attrs, and null for the other
// attributes.
func testPeering(attrs map[string]cty.Value) cty.Value {
	all := make(map[string]cty.Value)
	for name, typ := range peeringType().AttributeTypes() {
		all[name] = cty.NullVal(typ)
	}
	maps.Copy(all, attrs)

	return cty.ObjectVal(all)
}

// testConfig returns the configuration of a peering in region. A null cidrs
// is a configuration without peer_cidr_blocks: Terraform sends the same null
// for an omitted argument and for an explicit null.
func testConfig(region, cidrs cty.Value) map[string]cty.Value {
	return map[string]cty.Value{
		"cluster_id":       cty.NumberIntVal(42),
		"datacenter":       cty.StringVal("DC1"),
		"peer_vpc_id":      cty.StringVal("vpc-1"),
		"peer_cidr_blocks": cidrs,
		"peer_region":      region,
		"peer_account_id":  cty.StringVal("123"),
		"allow_cql":        cty.True,
	}
}

// testState returns the state of a peering in region with cidrs.
func testState(region string, cidrs ...string) map[string]cty.Value {
	attrs := testConfig(cty.StringVal(region), cidrList(cidrs...))
	attrs["id"] = cty.StringVal("pcx-1")
	attrs["vpc_peering_id"] = cty.NumberIntVal(9)
	attrs["connection_id"] = cty.StringVal("pcx-1")
	attrs["network_link"] = cty.StringVal("")

	return attrs
}

func cidrList(cidrs ...string) cty.Value {
	values := make([]cty.Value, len(cidrs))
	for i, cidr := range cidrs {
		values[i] = cty.StringVal(cidr)
	}

	return cty.ListVal(values)
}

// with returns a copy of attrs with name set to v.
func with(attrs map[string]cty.Value, name string, v cty.Value) map[string]cty.Value {
	out := maps.Clone(attrs)
	out[name] = v

	return out
}

func encode(t *testing.T, v cty.Value) *tfprotov5.DynamicValue {
	t.Helper()

	b, err := msgpack.Marshal(v, peeringType())
	require.NoError(t, err)

	return &tfprotov5.DynamicValue{MsgPack: b}
}

func decode(t *testing.T, v *tfprotov5.DynamicValue) cty.Value {
	t.Helper()

	got, err := msgpack.Unmarshal(v.MsgPack, peeringType())
	require.NoError(t, err)

	return got
}

// proposedNewState merges prior into config as Terraform does for this flat
// schema: a computed attribute that the configuration does not set keeps its
// prior value.
func proposedNewState(prior, config cty.Value) cty.Value {
	if prior.IsNull() {
		return config
	}

	attrs := config.AsValueMap()
	for name, attr := range ResourceVPCPeering().CoreConfigSchema().Attributes {
		if attr.Computed && attrs[name].IsNull() {
			attrs[name] = prior.GetAttr(name)
		}
	}

	return cty.ObjectVal(attrs)
}

// planPeering plans a peering through the provider server as Terraform does.
// A replacement is planned again from a null prior, as Terraform plans the
// new object, and so is forceReplace, as terraform plan -replace does.
func planPeering(t *testing.T, prior, config cty.Value, forceReplace bool) (planned cty.Value, replace bool, diags []*tfprotov5.Diagnostic) {
	t.Helper()

	server := testProviderServer(newTestClient(t, newNoAPI(t)))

	plan := func(prior cty.Value) *tfprotov5.PlanResourceChangeResponse {
		resp, err := server.PlanResourceChange(context.Background(), &tfprotov5.PlanResourceChangeRequest{
			TypeName:         testResourceType,
			PriorState:       encode(t, prior),
			ProposedNewState: encode(t, proposedNewState(prior, config)),
			Config:           encode(t, config),
		})
		require.NoError(t, err)

		return resp
	}

	resp := plan(prior)
	if len(resp.Diagnostics) > 0 {
		return cty.NilVal, false, resp.Diagnostics
	}

	replace = !prior.IsNull() && (len(resp.RequiresReplace) > 0 || forceReplace)
	if replace {
		resp = plan(cty.NullVal(peeringType()))
		if len(resp.Diagnostics) > 0 {
			return cty.NilVal, true, resp.Diagnostics
		}
	}

	return decode(t, resp.PlannedState), replace, nil
}

func TestResourceVPCPeeringPlan(t *testing.T) {
	t.Parallel()

	var (
		noCIDRs   = cty.NullVal(cty.List(cty.String))
		gcpState  = testState("us-central1", "10.128.0.0/20")
		gcpConfig = testConfig(cty.StringVal("us-central1"), noCIDRs)
		awsState  = testState("us-east-1", "192.168.0.0/16")
		awsConfig = testConfig(cty.StringVal("us-east-1"), cidrList("192.168.0.0/16"))

		unknownRegionErr = `"peer_cidr_blocks" is required when "peer_region" is known only at apply time`
	)

	tests := []struct {
		name string
		// prior is nil for a new peering.
		prior  map[string]cty.Value
		config map[string]cty.Value
		// forceReplace replaces the peering as terraform plan -replace does.
		forceReplace bool
		wantReplace  bool
		wantCIDRs    cty.Value
		wantErr      string
	}{
		{
			// The bug this plan fixes: the read stores the default, which
			// the configuration does not set.
			name:      "GCP default CIDR block not set in the configuration",
			prior:     gcpState,
			config:    gcpConfig,
			wantCIDRs: cidrList("10.128.0.0/20"),
		},
		{
			name:      "GCP default CIDR block with the region in uppercase",
			prior:     gcpState,
			config:    with(gcpConfig, "peer_region", cty.StringVal("US-CENTRAL1")),
			wantCIDRs: cidrList("10.128.0.0/20"),
		},
		{
			name:        "custom GCP CIDR blocks removed from the configuration",
			prior:       testState("us-central1", "10.10.0.0/16"),
			config:      gcpConfig,
			wantReplace: true,
			wantCIDRs:   cidrList("10.128.0.0/20"),
		},
		{
			name:    "AWS CIDR blocks removed from the configuration",
			prior:   awsState,
			config:  with(awsConfig, "peer_cidr_blocks", noCIDRs),
			wantErr: `"peer_cidr_blocks" is required: "us-east-1" has no default CIDR block`,
		},
		{
			name:    "GCP CIDR blocks removed in a region without a default",
			prior:   testState("me-central2", "10.10.0.0/16"),
			config:  testConfig(cty.StringVal("me-central2"), noCIDRs),
			wantErr: `"peer_cidr_blocks" is required: "me-central2" has no default CIDR block`,
		},
		{
			// The new peering must not keep the default of the old region.
			name:        "GCP region change without CIDR blocks",
			prior:       gcpState,
			config:      with(gcpConfig, "peer_region", cty.StringVal("europe-west1")),
			wantReplace: true,
			wantCIDRs:   cidrList("10.132.0.0/20"),
		},
		{
			name:        "GCP region change with CIDR blocks",
			prior:       testState("us-central1", "10.10.0.0/16"),
			config:      testConfig(cty.StringVal("europe-west1"), cidrList("10.10.0.0/16")),
			wantReplace: true,
			wantCIDRs:   cidrList("10.10.0.0/16"),
		},
		{
			name:      "same CIDR blocks",
			prior:     awsState,
			config:    awsConfig,
			wantCIDRs: cidrList("192.168.0.0/16"),
		},
		{
			name:        "other CIDR blocks",
			prior:       awsState,
			config:      with(awsConfig, "peer_cidr_blocks", cidrList("192.168.1.0/24")),
			wantReplace: true,
			wantCIDRs:   cidrList("192.168.1.0/24"),
		},
		{
			// An explicit unknown list stays unknown, it takes neither the
			// default nor the prior CIDR blocks.
			name:        "CIDR blocks known after apply",
			prior:       gcpState,
			config:      with(gcpConfig, "peer_cidr_blocks", cty.UnknownVal(cty.List(cty.String))),
			wantReplace: true,
			wantCIDRs:   cty.UnknownVal(cty.List(cty.String)),
		},
		{
			name:        "region and CIDR blocks known after apply",
			prior:       gcpState,
			config:      testConfig(cty.UnknownVal(cty.String), cty.UnknownVal(cty.List(cty.String))),
			wantReplace: true,
			wantCIDRs:   cty.UnknownVal(cty.List(cty.String)),
		},
		{
			// The unknown region replaces the peering, and the create
			// would fail after the delete.
			name:    "AWS CIDR blocks removed in a region known after apply",
			prior:   awsState,
			config:  testConfig(cty.UnknownVal(cty.String), noCIDRs),
			wantErr: unknownRegionErr,
		},
		{
			name:      "AWS region in uppercase",
			prior:     awsState,
			config:    with(awsConfig, "peer_region", cty.StringVal("US-EAST-1")),
			wantCIDRs: cidrList("192.168.0.0/16"),
		},
		{
			name:      "AWS region without clusters in uppercase",
			prior:     testState("eu-central-2", "192.168.0.0/16"),
			config:    with(awsConfig, "peer_region", cty.StringVal("EU-CENTRAL-2")),
			wantCIDRs: cidrList("192.168.0.0/16"),
		},
		{
			name:        "other AWS region",
			prior:       awsState,
			config:      with(awsConfig, "peer_region", cty.StringVal("eu-central-2")),
			wantReplace: true,
			wantCIDRs:   cidrList("192.168.0.0/16"),
		},
		{
			name:         "forced replacement keeps the CIDR blocks",
			prior:        awsState,
			config:       awsConfig,
			forceReplace: true,
			wantReplace:  true,
			wantCIDRs:    cidrList("192.168.0.0/16"),
		},
		{
			name:         "forced replacement plans the GCP default CIDR block",
			prior:        gcpState,
			config:       gcpConfig,
			forceReplace: true,
			wantReplace:  true,
			wantCIDRs:    cidrList("10.128.0.0/20"),
		},
		{
			name:      "new GCP peering without CIDR blocks",
			config:    gcpConfig,
			wantCIDRs: cidrList("10.128.0.0/20"),
		},
		{
			name:    "new AWS peering without CIDR blocks",
			config:  with(awsConfig, "peer_cidr_blocks", noCIDRs),
			wantErr: `"peer_cidr_blocks" is required: "us-east-1" has no default CIDR block`,
		},
		{
			name:      "new peering with CIDR blocks known after apply",
			config:    with(gcpConfig, "peer_cidr_blocks", cty.UnknownVal(cty.List(cty.String))),
			wantCIDRs: cty.UnknownVal(cty.List(cty.String)),
		},
		{
			// The SDK plans a list with an unknown element as unknown.
			name:      "new peering with a CIDR block known after apply",
			config:    with(gcpConfig, "peer_cidr_blocks", cty.ListVal([]cty.Value{cty.UnknownVal(cty.String)})),
			wantCIDRs: cty.UnknownVal(cty.List(cty.String)),
		},
		{
			name:    "new peering in a region known after apply without CIDR blocks",
			config:  with(gcpConfig, "peer_region", cty.UnknownVal(cty.String)),
			wantErr: unknownRegionErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			prior := cty.NullVal(peeringType())
			if tt.prior != nil {
				prior = testPeering(tt.prior)
			}

			planned, replace, diags := planPeering(t, prior, testPeering(tt.config), tt.forceReplace)

			if tt.wantErr != "" {
				require.Len(t, diags, 1)
				require.Equal(t, tfprotov5.DiagnosticSeverityError, diags[0].Severity)
				require.Equal(t, tt.wantErr, diags[0].Summary)
				return
			}

			require.Empty(t, diags)
			require.Equal(t, tt.wantReplace, replace)

			got := planned.GetAttr("peer_cidr_blocks")
			require.True(t, tt.wantCIDRs.RawEquals(got), "peer_cidr_blocks: want %#v, got %#v", tt.wantCIDRs, got)

			// An existing peering that is not replaced has no change at all.
			if tt.prior != nil && !tt.wantReplace {
				require.True(t, prior.RawEquals(planned), "unexpected change:\nprior   %#v\nplanned %#v", prior, planned)
			}
		})
	}
}

// TestResourceVPCPeeringRefresh refreshes through the provider server, which
// applies DiffSuppressOnRefresh.
func TestResourceVPCPeeringRefresh(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		stateRegion string
		apiRegion   string
		wantRegion  string
	}{
		{
			// The value of the configuration stays, so nothing that reads
			// it sees a change.
			name:        "region in another letter case",
			stateRegion: "US-EAST-1",
			apiRegion:   "us-east-1",
			wantRegion:  "US-EAST-1",
		},
		{
			name:        "other region",
			stateRegion: "us-east-1",
			apiRegion:   "eu-central-2",
			wantRegion:  "eu-central-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := newReadAPI(t, testCloudAWS, fmt.Sprintf(`"region":%q`, tt.apiRegion))
			server := testProviderServer(newTestClient(t, srv))

			resp, err := server.ReadResource(context.Background(), &tfprotov5.ReadResourceRequest{
				TypeName:     testResourceType,
				CurrentState: encode(t, testPeering(testState(tt.stateRegion, testAPICIDR))),
			})
			require.NoError(t, err)
			require.Empty(t, resp.Diagnostics)

			got := decode(t, resp.NewState).GetAttr("peer_region")
			require.Equal(t, cty.StringVal(tt.wantRegion), got)
		})
	}
}

func TestResourceVPCPeeringValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cidrs   cty.Value
		wantErr bool
	}{
		// Terraform sends a missing argument as null.
		{name: "CIDR blocks not set", cidrs: cty.NullVal(cty.List(cty.String))},
		{name: "CIDR blocks known after apply", cidrs: cty.UnknownVal(cty.List(cty.String))},
		{name: "CIDR blocks", cidrs: cty.ListVal([]cty.Value{cty.StringVal("192.168.0.0/16")})},
		{name: "empty CIDR block list", cidrs: cty.ListValEmpty(cty.String), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := ResourceVPCPeering()
			block := r.CoreConfigSchema()

			attrs := make(map[string]cty.Value)
			for name, typ := range block.ImpliedType().AttributeTypes() {
				attrs[name] = cty.NullVal(typ)
			}
			attrs["cluster_id"] = cty.NumberIntVal(42)
			attrs["datacenter"] = cty.StringVal("DC1")
			attrs["peer_vpc_id"] = cty.StringVal("vpc-1")
			attrs["peer_region"] = cty.StringVal("us-east-1")
			attrs["peer_account_id"] = cty.StringVal("123")
			attrs["peer_cidr_blocks"] = tt.cidrs

			diags := r.Validate(terraform.NewResourceConfigShimmed(cty.ObjectVal(attrs), block))

			if !tt.wantErr {
				require.Empty(t, diags)
				return
			}

			require.Len(t, diags, 1)
			require.Equal(t, "Not enough list items", diags[0].Summary)
			require.Equal(t, cty.GetAttrPath("peer_cidr_blocks"), diags[0].AttributePath)
		})
	}
}
