package provider

import (
	"context"
	"testing"
)

func TestProviderInternalValidate(t *testing.T) {
	t.Parallel()
	if err := New(context.Background()).InternalValidate(); err != nil {
		t.Fatalf("provider schema is invalid: %s", err)
	}
}
