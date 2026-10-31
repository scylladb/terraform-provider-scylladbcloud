package model

// VPCPeeringUpdateRequest changes a peering's status and peer CIDRs in place.
// CIDRBlock is a comma-separated list; empty keeps the current CIDRs.
type VPCPeeringUpdateRequest struct {
	Status    string `json:"status,omitempty"`
	CIDRBlock string `json:"cidrBlock,omitempty"`
}
