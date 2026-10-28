//go:build dev

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stretchr/testify/require"
)

func devResourceData(t *testing.T, raw map[string]any) *schema.ResourceData {
	t.Helper()
	return schema.TestResourceDataRaw(t, New(context.Background()).Schema, raw)
}

func TestDevSchema(t *testing.T) {
	p := New(context.Background())
	for _, name := range []string{"dev_mode", "dev_siren_endpoint", "dev_credentials_file"} {
		require.Contains(t, p.Schema, name)
	}
	require.Nil(t, p.Schema["token"].ValidateDiagFunc)
}

func TestDevModeEnvDefault(t *testing.T) {
	t.Setenv("SCYLLADB_CLOUD_DEV_MODE", "1")
	require.Equal(t, true, New(context.Background()).Schema["dev_mode"].Default)
}

func TestDevConfigureWithoutDevModeRequiresToken(t *testing.T) {
	d := devResourceData(t, map[string]any{"token": ""})
	_, diags := devConfigure(context.Background(), d, "http://127.0.0.1:1", "")
	require.True(t, diags.HasError())
	require.Equal(t, "token is required", diags[0].Summary)

	s, diags := devConfigure(context.Background(), d, "https://example.com", "tok")
	require.False(t, diags.HasError())
	require.Equal(t, devSettings{token: "tok"}, s)
}

func TestDevConfigureRefusesRemoteEndpoints(t *testing.T) {
	for _, tc := range []struct {
		name     string
		endpoint string
		siren    string
	}{
		{"production default", "https://api.cloud.scylladb.com", "http://127.0.0.1:8080/api/v1"},
		{"remote public api", "http://192.0.2.10:3011", "http://127.0.0.1:8080/api/v1"},
		{"remote siren", "http://127.0.0.1:3011", "http://192.0.2.10:8080/api/v1"},
		{"docker host", "http://host.docker.internal:3011", "http://127.0.0.1:8080/api/v1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := devResourceData(t, map[string]any{"dev_mode": true, "dev_siren_endpoint": tc.siren})
			_, diags := devConfigure(context.Background(), d, tc.endpoint, "tok")
			require.True(t, diags.HasError())
		})
	}
}

func TestDevConfigureLocalWithToken(t *testing.T) {
	d := devResourceData(t, map[string]any{"dev_mode": true, "dev_siren_endpoint": "http://127.0.0.1:8080/api/v1"})
	s, diags := devConfigure(context.Background(), d, "http://127.0.0.1:3011", "tok")
	require.False(t, diags.HasError())
	require.Len(t, diags, 1)
	require.Equal(t, diag.Warning, diags[0].Severity)
	require.Equal(t, "tok", s.token)
	require.Equal(t, " dev", s.userAgentSuffix)
	require.Len(t, s.opts, 2)
}
