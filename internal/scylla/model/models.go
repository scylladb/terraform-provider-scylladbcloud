package model

type UserAccount struct {
	UserID            int64  `json:"UserID"`
	AccountID         int64  `json:"AccountID"`
	Name              string `json:"Name"`
	OwnerUserID       int64  `json:"OwnerUserID"`
	AccountStatus     string `json:"AccountStatus"`
	Role              string `json:"Role"`
	UserAccountStatus string `json:"UserAccountStatus"`
}

type CloudProvider struct {
	ID            int64  `json:"ID"`
	Name          string `json:"Name"`
	RootAccountID string `json:"RootAccountID"`
}

type CloudProviderRegion struct {
	ID                          int64  `json:"ID"`
	CloudProviderID             int64  `json:"CloudProviderID"`
	Name                        string `json:"Name"`
	FullName                    string `json:"FullName"`
	ExternalID                  string `json:"ExternalID"`
	MultiRegionExternalID       string `json:"MultiRegionExternalID"`
	DcName                      string `json:"DCName"`
	BackupStorageGbCost         string `json:"BackupStorageGBCost"`
	TrafficSameRegionInGbCost   string `json:"TrafficSameRegionInGBCost"`
	TrafficSameRegionOutGbCost  string `json:"TrafficSameRegionOutGBCost"`
	TrafficCrossRegionOutGbCost string `json:"TrafficCrossRegionOutGBCost"`
	TrafficInternetOutGbCost    string `json:"TrafficInternetOutGBCost"`
	Continent                   string `json:"Continent"`
}

type DataCenter struct {
	ID                               int64  `json:"ID"`
	ClusterID                        int64  `json:"ClusterID"`
	CloudProviderID                  int64  `json:"CloudProviderID"`
	CloudProviderRegionID            int64  `json:"CloudProviderRegionID"`
	ReplicationFactor                int64  `json:"ReplicationFactor"`
	CIDR                             string `json:"IPv4CIDR"`
	AccountCloudProviderCredentialID int64  `json:"AccountCloudProviderCredentialID"`
	Status                           string `json:"Status"`
	Name                             string `json:"Name"`
	ManagementNetwork                string `json:"ManagementNetwork"`
	InstanceTypeID                   int64  `json:"InstanceTypeID"`
}

type DataCenterWithClientConnections struct {
	DataCenter
	ClientConnection []string `json:"ClientConnection"`
}

type FreeTier struct {
	ExpirationDate    string `json:"ExpirationDate"`
	ExpirationSeconds int64  `json:"ExpirationSeconds"`
	CreationTime      string `json:"CreationTime"`
}

type Cluster struct {
	ID                        int64                             `json:"ID"`
	Name                      string                            `json:"Name"`
	ClusterNameOnConfigFile   string                            `json:"ClusterNameOnConfigFile"`
	Status                    string                            `json:"Status"`
	CloudProviderID           int64                             `json:"CloudProviderID"`
	ReplicationFactor         int64                             `json:"ReplicationFactor"`
	BroadcastType             string                            `json:"BroadcastType"`
	ScyllaVersionID           int64                             `json:"ScyllaVersionID"`
	ScyllaVersion             string                            `json:"ScyllaVersion"`
	Dc                        []DataCenterWithClientConnections `json:"DC"`
	GrafanaURL                string                            `json:"GrafanaURL"`
	GrafanaRootURL            string                            `json:"GrafanaRootURL"`
	BackofficeGrafanaURL      string                            `json:"BackofficeGrafanaURL"`
	BackofficePrometheusURL   string                            `json:"BackofficePrometheusURL"`
	BackofficeAlertManagerURL string                            `json:"BackofficeAlertManagerURL"`
	FreeTier                  FreeTier                          `json:"FreeTier"`
	EncryptionMode            string                            `json:"EncryptionMode"`
	UserApiInterface          string                            `json:"UserAPIInterface"`
	PricingModel              int64                             `json:"PricingModel"`
	MaxAllowedCIDRRange       int64                             `json:"MaxAllowedCidrRange"`
	CreatedAt                 string                            `json:"CreatedAt"`
	DNS                       bool                              `json:"DNS"`
	PromProxyEnabled          bool                              `json:"PromProxyEnabled"`
}

type AllowlistRule struct {
	ID            int64  `json:"ID"`
	ClusterID     int64  `json:"ClusterID"`
	SourceAddress string `json:"SourceAddress"`
}

type Node struct {
	ID                          int64  `json:"ID"`
	ClusterID                   int64  `json:"ClusterID"`
	CloudProviderID             int64  `json:"CloudProviderID"`
	CloudProviderInstanceTypeID int64  `json:"CloudProviderInstanceTypeID"`
	CloudProviderRegionID       int64  `json:"CloudProviderRegionID"`
	PublicIP                    string `json:"PublicIP"`
	PrivateIP                   string `json:"PrivateIP"`
	ClusterJoinDate             string `json:"ClusterJoinDate"`
	ServiceID                   int64  `json:"ServiceID"`
	ServiceVersionID            int64  `json:"ServiceVersionID"`
	ServiceVersion              string `json:"ServiceVersion"`
	BillingStartDate            string `json:"BillingStartDate"`
	Hostname                    string `json:"Hostname"`
	ClusterHostID               string `json:"ClusterHostID"`
	Status                      string `json:"Status"`
	NodeState                   string `json:"NodeState"`
	ClusterDcID                 int64  `json:"ClusterDCID"`
	ServerActionID              int64  `json:"ServerActionID"`
	Distribution                string `json:"Distribution"`
	DNS                         string `json:"DNS"`
}

type VPC struct {
	ID                    int64  `json:"ID"`
	ClusterID             int64  `json:"ClusterID"`
	CloudProviderID       int64  `json:"CloudProviderID"`
	CloudProviderRegionID int64  `json:"CloudProviderRegionID"`
	ClusterDcID           int64  `json:"ClusterDCID"`
	CIDR                  string `json:"IPv4CIDR"`
}

type CloudProviderInstance struct {
	ID                         int64  `json:"ID"`
	CloudProviderID            int64  `json:"CloudProviderID"`
	Name                       string `json:"Name"`
	ExternalID                 string `json:"ExternalID"`
	MemoryMB                   int64  `json:"MemoryMB"`
	LocalDiskCount             int64  `json:"LocalDiskCount"`
	LocalStorageTotalGB        int64  `json:"LocalStorageTotalGB"`
	CPUCoreCount               int64  `json:"CPUCoreCount"`
	NetworkMBPS                int64  `json:"NetworkMBPS"`
	ExternalStorageNetworkMBPS int64  `json:"ExternalStorageNetworkMBPS"`
	Environment                string `json:"Environment"`
	DisplayOrder               int64  `json:"DisplayOrder"`
	NetworkSpeedDescription    string `json:"NetworkSpeedDescription"`
	LicenseCostOnDemandPerHour string `json:"LicenseCostOnDemandPerHour"`
	FreeTierHours              int64  `json:"FreeTierHours"`
	InstanceFamily             string `json:"InstanceFamily"`
	GroupDefault               bool   `json:"GroupDefault"`
	CostPerHour                string `json:"CostPerHour"`
	SubscriptionCostHourly     string `json:"SubscriptionCostHourly"`
	SubscriptionCostMonthly    string `json:"SubscriptionCostMonthly"`
	SubscriptionCostYearly     string `json:"SubscriptionCostYearly"`
}

type VPCPeering struct {
	ID                          int64    `json:"ID"`
	ExternalID                  string   `json:"ExternalID"`
	ClusterDCID                 int64    `json:"ClusterDCID"`
	PeerVPCIPv4CIDRList         []string `json:"PeerVPCIPv4CIDRList"`
	PeerVPCIPv4CIDRListVerified []string `json:"PeerVPCIPv4CIDRListVerified"`
	PeerVPCRegionID             int64    `json:"PeerVPCRegionID"`
	PeerVPCExternalID           string   `json:"PeerVPCExternalID"`
	PeerOwnerExternalID         string   `json:"PeerOwnerExternalID"`
	Status                      string   `json:"Status"`
	ExpiresAt                   string   `json:"ExpiresAt"`
	NetworkName                 string   `json:"NetworkName"`
	ProjectID                   string   `json:"ProjectID"`
}
