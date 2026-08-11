package cluster

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/eapache/go-resiliency/retrier"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
	"github.com/stretchr/testify/require"
)

func TestValidateMinNodesDiag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value int
		valid bool
	}{
		{name: "too small", value: 2},
		{name: "valid", value: 6, valid: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			diags := validateMinNodesDiag(tt.value, cty.Path{})
			if tt.valid {
				require.Nil(t, diags)
				return
			}

			require.NotNil(t, diags)
		})
	}
}

func TestValidateScalingTargetUtilizationDiag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value float64
		valid bool
	}{
		{name: "zero", value: 0},
		{name: "negative", value: -0.1},
		{name: "above one", value: 1.1},
		{name: "above max", value: 1.0},
		{name: "valid fractional", value: 0.75, valid: true},
		{name: "valid max", value: 0.9, valid: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			diags := validateScalingTargetUtilizationDiag(tt.value, cty.Path{})
			if tt.valid {
				require.Nil(t, diags)
				return
			}

			require.NotNil(t, diags)
		})
	}
}

func TestValidateScaling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		hasMinNodes   bool
		hasNodeType   bool
		scaling       map[string]interface{}
		valid         bool
		expectedError string
	}{
		{
			name:        "regular cluster valid",
			hasMinNodes: true,
			hasNodeType: true,
			valid:       true,
		},
		{
			name:          "regular cluster missing min nodes",
			hasNodeType:   true,
			expectedError: `"min_nodes" is required when the "scaling" block is not configured`,
		},
		{
			name:          "regular cluster missing node type",
			hasMinNodes:   true,
			expectedError: `"node_type" is required when the "scaling" block is not configured`,
		},
		{
			name:          "scaling missing selector",
			scaling:       map[string]interface{}{},
			expectedError: `exactly one of "instance_families" or "instance_types" must be configured in the "scaling" block`,
		},
		{
			name: "scaling both selectors",
			scaling: map[string]interface{}{
				"instance_families": []interface{}{"i4i"},
				"instance_types":    []interface{}{"i3.xlarge"},
			},
			expectedError: `exactly one of "instance_families" or "instance_types" must be configured in the "scaling" block`,
		},
		{
			name: "scaling with families valid",
			scaling: map[string]interface{}{
				"instance_families": []interface{}{"i4i"},
			},
			valid: true,
		},
		{
			name: "scaling with instance types valid",
			scaling: map[string]interface{}{
				"instance_types": []interface{}{"i3.large", "i3.xlarge"},
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateScaling(tt.hasMinNodes, tt.hasNodeType, tt.scaling)
			if tt.valid {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			if tt.expectedError != "" {
				require.EqualError(t, err, tt.expectedError)
			}
		})
	}
}

func TestExpandScaling(t *testing.T) {
	t.Parallel()

	t.Run("returns nil for empty block", func(t *testing.T) {
		t.Parallel()

		scaling, err := expandScaling(nil, "us-east-1", nil, nil)
		require.NoError(t, err)
		require.Nil(t, scaling)
	})

	t.Run("expands families and policies", func(t *testing.T) {
		t.Parallel()

		scaling, err := expandScaling([]interface{}{
			map[string]interface{}{
				"instance_families": []interface{}{"i4i"},
				"storage_policy": []interface{}{
					map[string]interface{}{
						"min_gb":             500,
						"target_utilization": 0.75,
					},
				},
				"vcpu_policy": []interface{}{
					map[string]interface{}{
						"min": 8,
					},
				},
			},
		}, "us-east-1", []model.CloudProviderInstance{
			{ID: 1, ExternalID: "i4i.large", Family: "i4i"},
		}, nil)
		require.NoError(t, err)

		require.Equal(t, &model.Scaling{
			InstanceFamilies: []string{"i4i"},
			Mode:             "xcloud",
			Policies: &model.ScalingPolicies{
				Storage: &model.ScalingStoragePolicy{
					Min:               500,
					TargetUtilization: 0.75,
				},
				VCPU: &model.ScalingVCPUPolicy{Min: 8},
			},
		}, scaling)
	})

	t.Run("expands instance types to ids", func(t *testing.T) {
		t.Parallel()

		instances := []model.CloudProviderInstance{
			{ID: 1, ExternalID: "i3.large"},
			{ID: 2, ExternalID: "i3.xlarge"},
		}

		scaling, err := expandScaling([]interface{}{
			map[string]interface{}{
				"instance_types": []interface{}{"i3.large", "i3.xlarge"},
			},
		}, "us-east-1", instances, &scylla.CloudProvider{})
		require.NoError(t, err)

		require.Equal(t, &model.Scaling{
			Mode:            "xcloud",
			InstanceTypeIDs: []int64{1, 2},
		}, scaling)
	})

	t.Run("returns error for unsupported instance type", func(t *testing.T) {
		t.Parallel()

		instances := []model.CloudProviderInstance{{ID: 1, ExternalID: "i3.large"}}

		scaling, err := expandScaling([]interface{}{
			map[string]interface{}{
				"instance_types": []interface{}{"m7i.large"},
			},
		}, "us-east-1", instances, &scylla.CloudProvider{})

		require.Nil(t, scaling)
		require.EqualError(t, err, `unsupported scaling instance_type "m7i.large" in region us-east-1`)
	})
}

func TestHasScaling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cluster *model.Cluster
		want    bool
	}{
		{name: "nil cluster"},
		{
			name: "scaling mode only",
			cluster: &model.Cluster{
				Datacenters: []model.Datacenter{
					{Scaling: &model.Scaling{InstanceFamilies: []string{"i4i"}}},
				},
			},
			want: true,
		},
		{
			name: "datacenter scaling",
			cluster: &model.Cluster{
				Datacenter: &model.Datacenter{
					Scaling: &model.Scaling{InstanceFamilies: []string{"i4i"}},
				},
			},
			want: true,
		},
		{
			name: "regular cluster",
			cluster: &model.Cluster{
				Datacenter: &model.Datacenter{},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, hasScaling(tt.cluster))
		})
	}
}

func TestFlattenScaling(t *testing.T) {
	t.Parallel()

	t.Run("returns empty for nil scaling", func(t *testing.T) {
		t.Parallel()

		got, err := flattenScaling(nil, nil, &scylla.CloudProvider{})
		require.NoError(t, err)
		require.Equal(t, []map[string]interface{}{}, got)
	})

	t.Run("maps instance type ids back to instance types", func(t *testing.T) {
		t.Parallel()

		instances := []model.CloudProviderInstance{
			{ID: 1, ExternalID: "i3.large"},
			{ID: 2, ExternalID: "i3.xlarge"},
		}

		got, err := flattenScaling(&model.Scaling{
			Mode:            model.ScalingXCloud,
			InstanceTypeIDs: []int64{2},
			Policies: &model.ScalingPolicies{
				Storage: &model.ScalingStoragePolicy{Min: 500, TargetUtilization: 0.75},
				VCPU:    &model.ScalingVCPUPolicy{Min: 8},
			},
		}, instances, &scylla.CloudProvider{})
		require.NoError(t, err)
		require.Equal(t, []map[string]interface{}{{
			"instance_types": []string{"i3.xlarge"},
			"storage_policy": []map[string]interface{}{{
				"min_gb":             500,
				"target_utilization": 0.75,
			}},
			"vcpu_policy": []map[string]interface{}{{
				"min": 8,
			}},
		}}, got)
	})

	t.Run("returns error for unknown instance id", func(t *testing.T) {
		t.Parallel()

		got, err := flattenScaling(&model.Scaling{
			Mode:            model.ScalingXCloud,
			InstanceTypeIDs: []int64{99},
		}, []model.CloudProviderInstance{{ID: 2, ExternalID: "i3.xlarge"}}, &scylla.CloudProvider{})
		require.Nil(t, got)
		require.EqualError(t, err, "unexpected scaling instance type ID 99")
	})
}

func TestSetClusterKVsSetsScaling(t *testing.T) {
	t.Parallel()

	resource := ResourceCluster()
	data := resource.TestResourceData()
	cluster := &model.Cluster{
		ID:               123,
		ClusterName:      "xcloud",
		UserAPIInterface: "CQL",
		BroadcastType:    "PRIVATE",
		DNS:              true,
		Status:           "ACTIVE",
		Region:           &model.CloudProviderRegion{ExternalID: "us-east-1"},
		ScyllaVersion:    &model.ScyllaVersion{Version: "2025.1"},
		// The API reports the instance the cluster currently runs on even for
		// X Cloud clusters; the provider must not track it.
		Instance: &model.CloudProviderInstance{ID: 2, ExternalID: "i3.xlarge", TotalStorage: 468},
		Datacenter: &model.Datacenter{
			Name:       "AWS_US_EAST_1",
			CIDRBlock:  "172.31.0.0/16",
			InstanceID: 2,
			Scaling: &model.Scaling{
				Mode:            model.ScalingXCloud,
				InstanceTypeIDs: []int64{2},
				Policies: &model.ScalingPolicies{
					Storage: &model.ScalingStoragePolicy{Min: 500, TargetUtilization: 0.75},
					VCPU:    &model.ScalingVCPUPolicy{Min: 8},
				},
			},
		},
		Datacenters: []model.Datacenter{{
			Scaling: &model.Scaling{
				Mode:            model.ScalingXCloud,
				InstanceTypeIDs: []int64{2},
				Policies: &model.ScalingPolicies{
					Storage: &model.ScalingStoragePolicy{Min: 500, TargetUtilization: 0.75},
					VCPU:    &model.ScalingVCPUPolicy{Min: 8},
				},
			},
		}},
	}
	instances := []model.CloudProviderInstance{{ID: 2, ExternalID: "i3.xlarge"}}

	err := setClusterKVs(data, cluster, "AWS", "", "", instances, &scylla.CloudProvider{})
	require.NoError(t, err)
	require.Equal(t, []interface{}{map[string]interface{}{
		"instance_families": []interface{}{},
		"instance_types":    []interface{}{"i3.xlarge"},
		"storage_policy": []interface{}{map[string]interface{}{
			"min_gb":             500,
			"target_utilization": 0.75,
		}},
		"vcpu_policy": []interface{}{map[string]interface{}{
			"min": 8,
		}},
	}}, data.Get("scaling"))
	require.Zero(t, data.Get("min_nodes"))
	require.Empty(t, data.Get("node_type"))
	require.Zero(t, data.Get("node_disk_size"))
}

// TestSetClusterKVsSetsInstanceForStandardCluster is the counterpart of
// TestSetClusterKVsSetsScaling: without a scaling block the instance the
// cluster runs on is part of the desired state and must be tracked.
func TestSetClusterKVsSetsInstanceForStandardCluster(t *testing.T) {
	t.Parallel()

	resource := ResourceCluster()
	data := resource.TestResourceData()
	cluster := &model.Cluster{
		ID:               123,
		ClusterName:      "standard",
		UserAPIInterface: "CQL",
		BroadcastType:    "PRIVATE",
		DNS:              true,
		Status:           "ACTIVE",
		Region:           &model.CloudProviderRegion{ExternalID: "us-east-1"},
		ScyllaVersion:    &model.ScyllaVersion{Version: "2025.1"},
		Instance:         &model.CloudProviderInstance{ID: 2, ExternalID: "i3.xlarge", TotalStorage: 468},
		Datacenter: &model.Datacenter{
			Name:       "AWS_US_EAST_1",
			CIDRBlock:  "172.31.0.0/16",
			InstanceID: 2,
		},
		Datacenters: []model.Datacenter{{}},
	}

	require.NoError(t, setClusterKVs(data, cluster, "AWS", "i3.xlarge", "", nil, &scylla.CloudProvider{}))
	require.Equal(t, "i3.xlarge", data.Get("node_type"))
	require.Equal(t, 468, data.Get("node_disk_size"))
	require.Empty(t, data.Get("scaling"))
}

func TestIsScalingEqual(t *testing.T) {
	t.Parallel()

	base := &model.Scaling{
		Mode:            model.ScalingXCloud,
		InstanceTypeIDs: []int64{2},
		Policies: &model.ScalingPolicies{
			Storage: &model.ScalingStoragePolicy{Min: 500, TargetUtilization: 0.75},
			VCPU:    &model.ScalingVCPUPolicy{Min: 8},
		},
	}

	tests := []struct {
		name string
		lhs  *model.Scaling
		rhs  *model.Scaling
		want bool
	}{
		{name: "both nil", want: true},
		{name: "same scaling", lhs: base, rhs: base, want: true},
		{
			name: "different instance type ids",
			lhs:  base,
			rhs: &model.Scaling{
				Mode:            model.ScalingXCloud,
				InstanceTypeIDs: []int64{3},
				Policies:        base.Policies,
			},
		},
		{
			name: "different storage min",
			lhs:  base,
			rhs: &model.Scaling{
				Mode:            model.ScalingXCloud,
				InstanceTypeIDs: []int64{2},
				Policies: &model.ScalingPolicies{
					Storage: &model.ScalingStoragePolicy{Min: 750, TargetUtilization: 0.75},
					VCPU:    &model.ScalingVCPUPolicy{Min: 8},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, isScalingEqual(tt.lhs, tt.rhs))
		})
	}
}

func TestValidateBackupRetentionDaysDiag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value int
		valid bool
	}{
		{name: "negative", value: -1},
		{name: "above max", value: 61},
		{name: "way above max", value: 100},
		{name: "zero", value: 0, valid: true},
		{name: "one (default)", value: 1, valid: true},
		{name: "max", value: 60, valid: true},
		{name: "mid range", value: 30, valid: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			diags := validateBackupRetentionDaysDiag(tt.value, cty.Path{})
			if tt.valid {
				require.Nil(t, diags)
				return
			}

			require.NotNil(t, diags)
		})
	}
}

func TestBackupRetentionDaysSchemaDefault(t *testing.T) {
	t.Parallel()

	resource := ResourceCluster()
	s, ok := resource.Schema["backup_retention_days"]
	require.True(t, ok, "backup_retention_days schema field must exist")
	require.Equal(t, 1, s.Default, "default must be 1 to prevent accidental data loss")
	require.True(t, s.Optional, "field must be optional")
}

func TestCACertificateSchema(t *testing.T) {
	t.Parallel()

	resource := ResourceCluster()
	s, ok := resource.Schema["ca_certificate"]
	require.True(t, ok, "ca_certificate schema field must exist")
	require.Equal(t, schema.TypeString, s.Type)
	require.True(t, s.Computed, "field must be computed")
	require.False(t, s.Optional, "field must not be optional")
	require.False(t, s.Sensitive, "public CA certificate is not sensitive")
}

func TestFetchCACertificate(t *testing.T) {
	t.Parallel()

	const pem = "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n"

	tests := []struct {
		name       string
		status     int
		body       string
		prev       string
		want       string
		wantDiags  int
		wantDetail string
	}{
		{
			name:   "success",
			status: http.StatusOK,
			body:   `{"data":{"format":"PEM","content":"` + strings.ReplaceAll(pem, "\n", `\n`) + `"}}`,
			prev:   "old",
			want:   pem,
		},
		{
			name:   "encryption disabled clears value",
			status: http.StatusOK, // the gateway responds 200 with an error body
			body:   `{"error":"041201"}`,
			prev:   "old",
			want:   "",
		},
		{
			name:   "encryption disabled clears value, direct API",
			status: http.StatusBadRequest,
			body:   `{"error":"041201"}`,
			prev:   "old",
			want:   "",
		},
		{
			name:       "generic failure keeps previous value with warning",
			status:     http.StatusInternalServerError,
			body:       `{"error":"041200"}`,
			prev:       "old",
			want:       "old",
			wantDiags:  1,
			wantDetail: "041200",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "/account/7/cluster/42/certificate", r.URL.Path)
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			endpoint, err := url.Parse(srv.URL)
			require.NoError(t, err)

			client := &scylla.Client{
				Endpoint:   endpoint,
				Headers:    make(http.Header),
				HTTPClient: srv.Client(),
				Retry:      retrier.New(nil, nil),
				AccountID:  7,
			}

			got, diags := fetchCACertificate(context.Background(), client, 42, tt.prev)
			require.Equal(t, tt.want, got)
			require.Len(t, diags, tt.wantDiags)
			if tt.wantDiags > 0 {
				require.Equal(t, diag.Warning, diags[0].Severity)
				require.Contains(t, diags[0].Detail, tt.wantDetail)
			}
		})
	}
}

func TestCreateClusterWithEncryptionFallback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// createError is the body the cluster POST answers with while it still
		// carries encryptionAtRest. Empty means the first attempt succeeds.
		createError string
		configured  bool
		wantPosts   []*model.EncryptionAtRest
		wantDiags   int
		wantErr     string
	}{
		{
			name:      "creates encrypted when the account is entitled",
			wantPosts: []*model.EncryptionAtRest{{Provider: "scylla-aws"}},
		},
		{
			name:        "retries unencrypted when the account is not entitled",
			createError: "040723",
			wantPosts:   []*model.EncryptionAtRest{{Provider: "scylla-aws"}, nil},
			wantDiags:   1,
		},
		{
			name:        "fails when the user asked for encryption explicitly",
			createError: "040723",
			configured:  true,
			wantPosts:   []*model.EncryptionAtRest{{Provider: "scylla-aws"}},
			wantErr:     "040723",
		},
		{
			name:        "never retries an invalid encryption parameter",
			createError: "040733",
			wantPosts:   []*model.EncryptionAtRest{{Provider: "scylla-aws"}},
			wantErr:     "040733",
		},
		{
			name:        "never retries an unrelated failure",
			createError: "040001",
			wantPosts:   []*model.EncryptionAtRest{{Provider: "scylla-aws"}},
			wantErr:     "040001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var posts []*model.EncryptionAtRest

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/account/7/cluster":
					var body model.ClusterCreateRequest
					require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
					posts = append(posts, body.EncryptionAtRest)

					if tt.createError != "" && body.EncryptionAtRest != nil {
						w.WriteHeader(http.StatusBadRequest)
						_, _ = w.Write([]byte(`{"error":"` + tt.createError + `"}`))
						return
					}
					_, _ = w.Write([]byte(`{"data":{"requestId":11}}`))
				case "/account/7/cluster/request/11":
					_, _ = w.Write([]byte(`{"data":{"id":11,"clusterId":42}}`))
				default:
					t.Errorf("unexpected path %q", r.URL.Path)
				}
			}))
			defer srv.Close()

			endpoint, err := url.Parse(srv.URL)
			require.NoError(t, err)

			client := &scylla.Client{
				Endpoint:   endpoint,
				Headers:    make(http.Header),
				HTTPClient: srv.Client(),
				Retry:      retrier.New(nil, nil),
				AccountID:  7,
			}

			req := &model.ClusterCreateRequest{
				ClusterName:      "encrypted-by-default",
				EncryptionAtRest: &model.EncryptionAtRest{Provider: model.EncryptionProviderScyllaAWS},
			}

			cr, diags, err := createClusterWithEncryptionFallback(context.Background(), client, req, tt.configured)
			require.Equal(t, tt.wantPosts, posts)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				require.Nil(t, cr)
				require.Empty(t, diags)
				return
			}

			require.NoError(t, err)
			require.Equal(t, int64(42), cr.ClusterID)
			require.Len(t, diags, tt.wantDiags)
			if tt.wantDiags > 0 {
				require.Equal(t, diag.Warning, diags[0].Severity)
				require.Contains(t, diags[0].Detail, "not entitled to encryption at rest")
			}
		})
	}
}

func TestSetClusterKVsSetsCACertificate(t *testing.T) {
	t.Parallel()

	const pem = "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n"

	resource := ResourceCluster()
	data := resource.TestResourceData()
	cluster := &model.Cluster{
		ID:               123,
		ClusterName:      "encrypted",
		UserAPIInterface: "CQL",
		BroadcastType:    "PRIVATE",
		DNS:              true,
		Status:           "ACTIVE",
		Region:           &model.CloudProviderRegion{ExternalID: "us-east-1"},
		ScyllaVersion:    &model.ScyllaVersion{Version: "2025.1"},
		Datacenter: &model.Datacenter{
			Name:      "AWS_US_EAST_1",
			CIDRBlock: "172.31.0.0/16",
		},
		Datacenters: []model.Datacenter{{}},
	}

	err := setClusterKVs(data, cluster, "AWS", "", pem, nil, &scylla.CloudProvider{})
	require.NoError(t, err)
	require.Equal(t, pem, data.Get("ca_certificate"))
}

func TestExpandEncryptionAtRest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		cloud string
		raw   interface{}
		want  *model.EncryptionAtRest
	}{
		{
			name:  "defaults to a scylla managed key for aws when the block is absent",
			cloud: "AWS",
			raw:   []interface{}{},
			want:  &model.EncryptionAtRest{Provider: "scylla-aws"},
		},
		{
			name:  "defaults to a scylla managed key for gcp when the block is absent",
			cloud: "GCP",
			raw:   []interface{}{},
			want:  &model.EncryptionAtRest{Provider: "scylla-gcp"},
		},
		{
			// The user did not ask for encryption, so an unsupported cloud must
			// not start failing creates.
			name:  "returns nil for an absent block on an unsupported cloud",
			cloud: "AZURE",
			raw:   []interface{}{},
			want:  nil,
		},
		{
			name:  "returns nil when disabled",
			cloud: "AWS",
			raw:   []interface{}{map[string]interface{}{"enabled": false, "key_id": "", "provider": ""}},
			want:  nil,
		},
		{
			name:  "derives scylla managed key provider for aws",
			cloud: "AWS",
			raw:   []interface{}{map[string]interface{}{"enabled": true, "key_id": "", "provider": ""}},
			want:  &model.EncryptionAtRest{Provider: "scylla-aws"},
		},
		{
			name:  "derives scylla managed key provider for gcp",
			cloud: "GCP",
			raw:   []interface{}{map[string]interface{}{"enabled": true, "key_id": "", "provider": ""}},
			want:  &model.EncryptionAtRest{Provider: "scylla-gcp"},
		},
		{
			name:  "derives customer managed key provider for aws",
			cloud: "AWS",
			raw:   []interface{}{map[string]interface{}{"enabled": true, "key_id": "key-deadbeef", "provider": ""}},
			want:  &model.EncryptionAtRest{Provider: "aws", KeyID: "key-deadbeef"},
		},
		{
			name:  "derives customer managed key provider for gcp",
			cloud: "GCP",
			raw:   []interface{}{map[string]interface{}{"enabled": true, "key_id": "key-deadbeef", "provider": ""}},
			want:  &model.EncryptionAtRest{Provider: "gcp", KeyID: "key-deadbeef"},
		},
		{
			name:  "accepts lowercase cloud",
			cloud: "aws",
			raw:   []interface{}{map[string]interface{}{"enabled": true, "key_id": "", "provider": ""}},
			want:  &model.EncryptionAtRest{Provider: "scylla-aws"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := expandEncryptionAtRest(tt.cloud, tt.raw)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}

	t.Run("returns error for unsupported cloud", func(t *testing.T) {
		t.Parallel()

		got, err := expandEncryptionAtRest("AZURE", []interface{}{
			map[string]interface{}{"enabled": true, "key_id": "", "provider": ""},
		})
		require.Nil(t, got)
		require.EqualError(t, err, `encryption at rest is not supported for cloud "AZURE"`)
	})
}

func TestFlattenEncryptionAtRest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  *model.EncryptionAtRest
		want []map[string]interface{}
	}{
		{
			name: "unencrypted reads back as disabled rather than an empty list",
			raw:  nil,
			want: []map[string]interface{}{{"enabled": false, "key_id": "", "provider": ""}},
		},
		{
			name: "encrypted",
			raw:  &model.EncryptionAtRest{Provider: "scylla-aws", KeyID: "key-T10FDq19E1b8Vukm"},
			want: []map[string]interface{}{{"enabled": true, "key_id": "key-T10FDq19E1b8Vukm", "provider": "scylla-aws"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, flattenEncryptionAtRest(tt.raw))
		})
	}
}

func TestValidateEncryptionAtRest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cloud   string
		enabled bool
		keyID   string
		wantErr string
	}{
		{
			name:    "enabled without key",
			cloud:   "AWS",
			enabled: true,
		},
		{
			name:    "enabled with key",
			cloud:   "GCP",
			enabled: true,
			keyID:   "key-deadbeef",
		},
		{
			name:  "disabled",
			cloud: "AWS",
		},
		{
			name:    "disabled with key",
			cloud:   "AWS",
			keyID:   "key-deadbeef",
			wantErr: `"key_id" cannot be set when "enabled" is false in the "encryption_at_rest" block`,
		},
		{
			name:    "malformed key",
			cloud:   "AWS",
			enabled: true,
			keyID:   "deadbeef",
			wantErr: `invalid "key_id" "deadbeef" in the "encryption_at_rest" block, expected a portal key ID such as "key-deadbeef"`,
		},
		{
			name:    "unsupported cloud",
			cloud:   "AZURE",
			enabled: true,
			wantErr: `encryption at rest is not supported for cloud "AZURE"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateEncryptionAtRest(tt.cloud, tt.enabled, tt.keyID)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestSetClusterKVsSetsEncryptionAtRest(t *testing.T) {
	t.Parallel()

	resource := ResourceCluster()
	data := resource.TestResourceData()
	cluster := &model.Cluster{
		ID:               123,
		ClusterName:      "encrypted-at-rest",
		UserAPIInterface: "CQL",
		BroadcastType:    "PRIVATE",
		DNS:              true,
		Status:           "ACTIVE",
		Region:           &model.CloudProviderRegion{ExternalID: "us-east-1"},
		ScyllaVersion:    &model.ScyllaVersion{Version: "2025.1"},
		Datacenter: &model.Datacenter{
			Name:      "AWS_US_EAST_1",
			CIDRBlock: "172.31.0.0/16",
		},
		Datacenters:      []model.Datacenter{{}},
		EncryptionAtRest: &model.EncryptionAtRest{Provider: "scylla-aws", KeyID: "key-T10FDq19E1b8Vukm"},
	}

	require.NoError(t, setClusterKVs(data, cluster, "AWS", "", "", nil, &scylla.CloudProvider{}))
	require.Equal(t, []interface{}{map[string]interface{}{
		"enabled":  true,
		"key_id":   "key-T10FDq19E1b8Vukm",
		"provider": "scylla-aws",
	}}, data.Get("encryption_at_rest"))
}

// TestConfiguredEncryptionAtRest covers the branches TestEncryptionAtRestPlan
// cannot reach, notably the null configuration of a destroy plan.
func TestConfiguredEncryptionAtRest(t *testing.T) {
	t.Parallel()

	blockType := cty.Object(map[string]cty.Type{
		"enabled":  cty.Bool,
		"key_id":   cty.String,
		"provider": cty.String,
	})
	configType := cty.Object(map[string]cty.Type{
		"encryption_at_rest": cty.List(blockType),
	})

	config := func(blocks cty.Value) cty.Value {
		return cty.ObjectVal(map[string]cty.Value{"encryption_at_rest": blocks})
	}
	block := func(keyID cty.Value) cty.Value {
		return cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{
			"enabled":  cty.True,
			"key_id":   keyID,
			"provider": cty.NullVal(cty.String),
		})})
	}

	tests := []struct {
		name           string
		config         cty.Value
		wantKeyID      string
		wantConfigured bool
	}{
		{
			name:   "null config, as on a destroy plan",
			config: cty.NullVal(configType),
		},
		{
			name:   "no block",
			config: config(cty.ListValEmpty(blockType)),
		},
		{
			name:           "block without key_id",
			config:         config(block(cty.NullVal(cty.String))),
			wantConfigured: true,
		},
		{
			name:           "block with key_id",
			config:         config(block(cty.StringVal("key-deadbeef"))),
			wantKeyID:      "key-deadbeef",
			wantConfigured: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			keyID, configured := configuredEncryptionAtRest(tt.config)
			require.Equal(t, tt.wantKeyID, keyID)
			require.Equal(t, tt.wantConfigured, configured)
		})
	}
}

// clusterDiff runs the resource through the real diff path, including
// CustomizeDiff, from the given cty configuration values. Attributes that are
// not listed are left null, mirroring what Terraform sends for an unset field.
func clusterDiff(t *testing.T, state *terraform.InstanceState, values map[string]cty.Value) (*terraform.InstanceDiff, error) {
	t.Helper()

	resource := ResourceCluster()
	block := resource.CoreConfigSchema()

	attrs := map[string]cty.Value{}
	for name, ty := range block.ImpliedType().AttributeTypes() {
		if v, ok := values[name]; ok {
			attrs[name] = v
			continue
		}
		attrs[name] = cty.NullVal(ty)
	}

	value := cty.ObjectVal(attrs)
	config := terraform.NewResourceConfigShimmed(value, block)

	// Mirror what PlanResourceChange does, so CustomizeDiff sees the same raw
	// configuration it sees in production.
	if state == nil {
		state = &terraform.InstanceState{}
	}
	state.RawConfig = value

	return resource.Diff(context.Background(), state, config, nil)
}

// TestEncryptionAtRestPlan drives the real diff path, including CustomizeDiff.
// It pins down the behaviours the attribute's design depends on: Default on a
// nested block attribute, Optional+Computed suppressing the diff for an omitted
// block, and ForceNew reaching the nested fields.
func TestEncryptionAtRestPlan(t *testing.T) {
	t.Parallel()

	encryptionAtRest := func(enabled, keyID cty.Value) cty.Value {
		return cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{
			"enabled":  enabled,
			"key_id":   keyID,
			"provider": cty.NullVal(cty.String),
		})})
	}

	baseConfig := map[string]cty.Value{
		"name":      cty.StringVal("cluster"),
		"cloud":     cty.StringVal("AWS"),
		"region":    cty.StringVal("us-east-1"),
		"node_type": cty.StringVal("i3.large"),
		"min_nodes": cty.NumberIntVal(3),
	}

	withEncryptionAtRest := func(block cty.Value) map[string]cty.Value {
		values := map[string]cty.Value{"encryption_at_rest": block}
		for k, v := range baseConfig {
			values[k] = v
		}
		return values
	}

	t.Run("enabled defaults to true when the block is present", func(t *testing.T) {
		t.Parallel()

		diff, err := clusterDiff(t, nil, withEncryptionAtRest(
			encryptionAtRest(cty.NullVal(cty.Bool), cty.NullVal(cty.String)),
		))
		require.NoError(t, err)
		require.Equal(t, "true", diff.Attributes["encryption_at_rest.0.enabled"].New)
	})

	t.Run("key_id alone still enables encryption", func(t *testing.T) {
		t.Parallel()

		diff, err := clusterDiff(t, nil, withEncryptionAtRest(
			encryptionAtRest(cty.NullVal(cty.Bool), cty.StringVal("key-deadbeef")),
		))
		require.NoError(t, err)
		require.Equal(t, "true", diff.Attributes["encryption_at_rest.0.enabled"].New)
		require.Equal(t, "key-deadbeef", diff.Attributes["encryption_at_rest.0.key_id"].New)
	})

	t.Run("replacement create plans key_id as unknown, not the old key", func(t *testing.T) {
		t.Parallel()

		// A nil prior state is what Terraform sends for the create half of a
		// replacement: it re-plans with a null prior so that computed values
		// from the doomed object are not carried over.
		diff, err := clusterDiff(t, nil, withEncryptionAtRest(
			encryptionAtRest(cty.True, cty.NullVal(cty.String)),
		))
		require.NoError(t, err)

		keyID := diff.Attributes["encryption_at_rest.0.key_id"]
		require.NotNil(t, keyID)
		require.True(t, keyID.NewComputed,
			"a ScyllaDB-managed key must never be resent as a customer-managed one")
		require.Empty(t, keyID.New)
	})

	encryptedState := func() *terraform.InstanceState {
		return &terraform.InstanceState{
			ID: "42",
			Attributes: map[string]string{
				"id":                            "42",
				"encryption_at_rest.#":          "1",
				"encryption_at_rest.0.enabled":  "true",
				"encryption_at_rest.0.key_id":   "key-deadbeef",
				"encryption_at_rest.0.provider": "scylla-aws",
				"name":                          "cluster",
				"cloud":                         "AWS",
				"region":                        "us-east-1",
				"node_type":                     "i3.large",
				"min_nodes":                     "3",
			},
		}
	}

	t.Run("omitting the block does not plan a replacement", func(t *testing.T) {
		t.Parallel()

		diff, err := clusterDiff(t, encryptedState(), baseConfig)
		require.NoError(t, err)

		// Optional+Computed means the block is reported as "known after
		// apply" rather than removed. Anything else would replace the cluster
		// for every user who has not adopted the new attribute.
		for key, attr := range diff.Attributes {
			if !strings.HasPrefix(key, "encryption_at_rest") {
				continue
			}
			require.Equal(t, "encryption_at_rest.#", key, "no per-field diff expected")
			require.True(t, attr.NewComputed)
			require.False(t, attr.RequiresNew, "omitting the block must not replace the cluster")
			require.False(t, attr.NewRemoved)
		}
	})

	t.Run("flipping enabled forces a replacement", func(t *testing.T) {
		t.Parallel()

		diff, err := clusterDiff(t, encryptedState(), withEncryptionAtRest(
			encryptionAtRest(cty.False, cty.NullVal(cty.String)),
		))
		// NoError also matters here: key_id is Optional+Computed, so the diff
		// still carries the key the API reported. Validating it rather than the
		// raw configuration would reject this as "key_id with enabled = false".
		require.NoError(t, err)

		enabled := diff.Attributes["encryption_at_rest.0.enabled"]
		require.NotNil(t, enabled)
		require.Equal(t, "true", enabled.Old)
		require.Equal(t, "false", enabled.New)
		require.True(t, enabled.RequiresNew, "encryption at rest is create-time only")
	})

	t.Run("changing key_id forces a replacement", func(t *testing.T) {
		t.Parallel()

		diff, err := clusterDiff(t, encryptedState(), withEncryptionAtRest(
			encryptionAtRest(cty.NullVal(cty.Bool), cty.StringVal("key-cafebabe")),
		))
		require.NoError(t, err)

		keyID := diff.Attributes["encryption_at_rest.0.key_id"]
		require.NotNil(t, keyID)
		require.Equal(t, "key-deadbeef", keyID.Old)
		require.Equal(t, "key-cafebabe", keyID.New)
		require.True(t, keyID.RequiresNew, "encryption at rest is create-time only")
	})

	t.Run("removing a customer managed key from the block is reported", func(t *testing.T) {
		t.Parallel()

		state := encryptedState()
		state.Attributes["encryption_at_rest.0.provider"] = "aws"

		_, err := clusterDiff(t, state, withEncryptionAtRest(
			encryptionAtRest(cty.True, cty.NullVal(cty.String)),
		))
		require.ErrorContains(t, err, `"key_id" is missing from the "encryption_at_rest" block`)
	})

	t.Run("omitting the whole block on a customer managed cluster stays a no-op", func(t *testing.T) {
		t.Parallel()

		state := encryptedState()
		state.Attributes["encryption_at_rest.0.provider"] = "aws"

		_, err := clusterDiff(t, state, baseConfig)
		require.NoError(t, err)
	})

	t.Run("malformed key_id is rejected at plan time", func(t *testing.T) {
		t.Parallel()

		_, err := clusterDiff(t, nil, withEncryptionAtRest(
			encryptionAtRest(cty.NullVal(cty.Bool), cty.StringVal("deadbeef")),
		))
		require.ErrorContains(t, err, `invalid "key_id" "deadbeef"`)
	})
}
