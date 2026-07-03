package model

// VectorSearchRequest is the request body for installing (POST) and resizing (PATCH) vector search.
type VectorSearchRequest struct {
	NodeCount             int   `json:"nodeCount"`
	DefaultInstanceTypeID int64 `json:"defaultInstanceTypeId"`
}

// VectorSearchInfo is the response from GET /account/{}/cluster/{}/dc/{}/vector-search.
type VectorSearchInfo struct {
	Status            string           `json:"status"`
	AvailabilityZones []VectorSearchAZ `json:"availabilityZones"`
}

// VectorSearchAZ describes an availability zone with vector search nodes.
type VectorSearchAZ struct {
	AZID     string             `json:"azid"`
	RackName string             `json:"rackName"`
	Nodes    []VectorSearchNode `json:"nodes"`
}

// VectorSearchNode describes a single vector search node.
type VectorSearchNode struct {
	ID             int64  `json:"id"`
	InstanceTypeID int64  `json:"instanceTypeId"`
	Status         string `json:"status"`
	Up             bool   `json:"up"`
	IndexCount     int    `json:"indexCount"`
}
