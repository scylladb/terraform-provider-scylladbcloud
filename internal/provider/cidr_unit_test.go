package provider

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCIDRIndexIsUnique(t *testing.T) {
	t.Parallel()
	seen := map[int]string{}
	for purpose, i := range testAccCIDRIndex {
		require.True(t, i > 0 && i < 256, "%s: index %d out of range", purpose, i)
		require.Empty(t, seen[i], "%s and %s share a CIDR block", purpose, seen[i])
		seen[i] = purpose
	}
	require.Equal(t, "10.0.3.0/24", testAccCIDR(testAccPrefix+"basic-aws-1234567890"))
}
