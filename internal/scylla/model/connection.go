package model

// ClusterConnectionRecreateRequest replaces a connection's whole revision:
// omitted fields are sent empty and an empty Status means ACTIVE.
type ClusterConnectionRecreateRequest struct {
	Name     string            `json:"name"`
	CIDRList []string          `json:"cidrList"`
	Data     map[string]string `json:"data,omitempty"`
	Status   string            `json:"status,omitempty"`
}
