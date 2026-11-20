package vpcpeering

import "testing"

func TestHistoricalSchemaInternalValidate(t *testing.T) {
	t.Parallel()
	if err := resourceVPCPeeringV0().InternalValidate(nil, true); err != nil {
		t.Fatalf("vpc peering schema v0 is invalid: %s", err)
	}
}
