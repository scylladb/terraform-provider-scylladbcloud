package model

// Vector search status values.
const (
	VectorSearchStatusHealthy   = "HEALTHY"
	VectorSearchStatusDegraded  = "DEGRADED"
	VectorSearchStatusUnhealthy = "UNHEALTHY"
	VectorSearchStatusUnknown   = "UNKNOWN"
)

type VectorSearchRequest struct {
	NodeCount             int64 `json:"nodeCount"`
	DefaultInstanceTypeID int64 `json:"defaultInstanceTypeId"`
}

type VectorSearch struct {
	Status            string             `json:"status"`
	AvailabilityZones []VectorSearchZone `json:"availabilityZones"`
}

type VectorSearchZone struct {
	AZID     string             `json:"azid"`
	RackName string             `json:"rackName"`
	Nodes    []VectorSearchNode `json:"nodes"`
}

type VectorSearchNode struct {
	ID             int64  `json:"id"`
	InstanceTypeID int64  `json:"instanceTypeId"`
	Status         string `json:"status"`
	Up             bool   `json:"up"`
}
