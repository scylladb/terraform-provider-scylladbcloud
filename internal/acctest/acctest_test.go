package acctest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckEnv(t *testing.T) {
	tests := []struct {
		name, endpoint, token, devMode, wantErr string
	}{
		{name: "no endpoint", token: "t", wantErr: "SCYLLADB_CLOUD_ENDPOINT must be set"},
		{name: "no token", endpoint: "http://localhost:3011", wantErr: "SCYLLADB_CLOUD_TOKEN must be set"},
		{name: "token", endpoint: "http://localhost:3011", token: "t"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SCYLLADB_CLOUD_ENDPOINT", tt.endpoint)
			t.Setenv("SCYLLADB_CLOUD_TOKEN", tt.token)
			t.Setenv("SCYLLADB_CLOUD_DEV_MODE", tt.devMode)
			err := checkEnv()
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestDevModeWithoutTokenNeedsDevBuild(t *testing.T) {
	t.Setenv("SCYLLADB_CLOUD_ENDPOINT", "http://localhost:3011")
	t.Setenv("SCYLLADB_CLOUD_TOKEN", "")
	t.Setenv("SCYLLADB_CLOUD_DEV_MODE", "1")
	err := checkEnv()
	if devBuild {
		require.NoError(t, err)
	} else {
		require.ErrorContains(t, err, "SCYLLADB_CLOUD_TOKEN must be set")
	}
}
