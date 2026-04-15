package cluster

import (
	"testing"

	"github.com/hashicorp/go-cty/cty"
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
		{name: "valid fractional", value: 0.75, valid: true},
		{name: "valid one", value: 1.0, valid: true},
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
		name        string
		hasScaling  bool
		hasMinNodes bool
		hasNodeType bool
		scaling     map[string]interface{}
		valid       bool
	}{
		{
			name:        "regular cluster valid",
			hasMinNodes: true,
			hasNodeType: true,
			valid:       true,
		},
		{
			name:        "regular cluster missing min nodes",
			hasNodeType: true,
		},
		{
			name:        "regular cluster missing node type",
			hasMinNodes: true,
		},
		{
			name:        "scaling conflicts with min nodes",
			hasScaling:  true,
			hasMinNodes: true,
			scaling: map[string]interface{}{
				"instance_families": []interface{}{"i4i"},
			},
		},
		{
			name:        "scaling conflicts with node type",
			hasScaling:  true,
			hasNodeType: true,
			scaling: map[string]interface{}{
				"instance_families": []interface{}{"i4i"},
			},
		},
		{
			name:       "scaling missing selector",
			hasScaling: true,
			scaling:    map[string]interface{}{},
		},
		{
			name:       "scaling both selectors",
			hasScaling: true,
			scaling: map[string]interface{}{
				"instance_families": []interface{}{"i4i"},
				"instance_types":    []interface{}{"i3.xlarge"},
			},
		},
		{
			name:       "scaling with families valid",
			hasScaling: true,
			scaling: map[string]interface{}{
				"instance_families": []interface{}{"i4i"},
			},
			valid: true,
		},
		{
			name:       "scaling with instance types valid",
			hasScaling: true,
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
