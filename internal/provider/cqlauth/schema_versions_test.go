package cqlauth

import "testing"

func TestHistoricalSchemaInternalValidate(t *testing.T) {
	t.Parallel()
	if err := dataSourceCQLAuthV0().InternalValidate(nil, false); err != nil {
		t.Fatalf("cql auth schema v0 is invalid: %s", err)
	}
}
