package scylla

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewTrace(t *testing.T) {
	trace, err := NewTrace()
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(trace, TracePrefix), "trace %q missing prefix %q", trace, TracePrefix)

	raw, err := hex.DecodeString(strings.TrimPrefix(trace, TracePrefix))
	require.NoError(t, err)
	require.Len(t, raw, 16)

	other, err := NewTrace()
	require.NoError(t, err)
	require.NotEqual(t, trace, other)
}

func TestTraceOrNew(t *testing.T) {
	t.Run("configured trace is preserved", func(t *testing.T) {
		trace, err := TraceOrNew("explicit")
		require.NoError(t, err)
		require.Equal(t, "explicit", trace)
	})

	t.Run("missing trace is generated", func(t *testing.T) {
		trace, err := TraceOrNew("")
		require.NoError(t, err)
		require.True(t, strings.HasPrefix(trace, TracePrefix),
			"trace %q missing prefix %q", trace, TracePrefix)
	})
}

func TestWithTrace(t *testing.T) {
	t.Run("sets header", func(t *testing.T) {
		req := New(WithTrace("trace-id")).Request(t.Context(), "GET", nil, "/")
		require.Equal(t, "trace-id", req.Header.Get(TraceHeader))
	})

	t.Run("empty trace is a no-op", func(t *testing.T) {
		req := New(WithTrace("")).Request(t.Context(), "GET", nil, "/")
		require.Empty(t, req.Header.Values(TraceHeader))
	})
}
