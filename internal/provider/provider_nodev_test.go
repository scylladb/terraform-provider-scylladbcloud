//go:build !dev

package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/stretchr/testify/require"
)

func TestReleaseSchemaHasNoDevAttributes(t *testing.T) {
	p := New(context.Background())
	for name := range p.Schema {
		require.False(t, strings.HasPrefix(name, "dev_"), "release build exposes %s", name)
	}
	require.ElementsMatch(t, []string{"endpoint", "token", "metadata", "trace"}, keys(p.Schema))
}

func TestReleaseTokenIsValidated(t *testing.T) {
	p := New(context.Background())
	validate := p.Schema["token"].ValidateDiagFunc
	require.NotNil(t, validate)

	diags := validate("", cty.Path{})
	require.Len(t, diags, 1)
	require.Equal(t, "token is required", diags[0].Summary)
	require.Nil(t, validate("tok", cty.Path{}))
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
