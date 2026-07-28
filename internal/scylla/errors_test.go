package scylla

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsEncryptionDisabledErr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil error"},
		{name: "plain error", err: errors.New("boom")},
		{name: "other api error", err: &APIError{Code: "041200"}},
		{name: "encryption disabled", err: &APIError{Code: "041201"}, want: true},
		{name: "wrapped encryption disabled", err: fmt.Errorf("read cluster: %w", &APIError{Code: "041201"}), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, IsEncryptionDisabledErr(tt.err))
		})
	}
}
