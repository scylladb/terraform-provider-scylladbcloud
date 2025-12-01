package model

import (
	"encoding/json"
	"strings"
	"time"
)

type CloudProvider struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	RootAccountID string `json:"rootAccountID"`
}

type CloudProviders struct {
	CloudProviders []CloudProvider `json:"cloudProviders"`
}

type ScyllaVersion struct {
	VersionID   int64  `json:"id"`
	Version     string `json:"version"`
	Description string `json:"description"`
	NewCluster  string `json:"newCluster"`
}

type ScyllaVersions struct {
	DefaultScyllaVersionID int64           `json:"defaultScyllaVersionId"`
	ScyllaVersions         []ScyllaVersion `json:"scyllaVersions"`
}

type CloudProviderRegion struct {
	ID                          int64       `json:"id"`
	ExternalID                  string      `json:"externalId"`
	CloudProviderID             int64       `json:"cloudProviderId"`
	Name                        string      `json:"name"`
	DatacenterName              string      `json:"dcName"`
	FullName                    string      `json:"fullName"`
	Continent                   string      `json:"continent"`
	BackupStorageGBCost         json.Number `json:"backupStorageGBCost"`
	TrafficSameRegionInGBCost   json.Number `json:"trafficSameRegionInGBCost"`
	TrafficSameRegionOutGBCost  json.Number `json:"trafficSameRegionOutGBCost"`
	TrafficCrossRegionOutGBCost json.Number `json:"trafficCrossRegionOutGBCost"`
	TrafficInternetOutGBCost    json.Number `json:"trafficInternetOutGBCost"`
}

type CloudProviderInstance struct {
	ID                          int64       `json:"id"`
	ExternalID                  string      `json:"externalId"`
	CloudProviderID             int64       `json:"cloudProviderId"`
	GroupDefault                bool        `json:"groupDefault"`
	DisplayOrder                int64       `json:"displayOrder"`
	Memory                      int64       `json:"memory"`
	LocalDiskCount              int64       `json:"localDiskCount"`
	TotalStorage                int64       `json:"totalStorage"`
	CPUCount                    int64       `json:"cpuCount"`
	NetworkSpeed                int64       `json:"networkSpeed"`
	ExternalStorageNetworkSpeed int64       `json:"externalStorageNetworkSpeed"`
	CostPerHour                 json.Number `json:"costPerHour"`
	Environment                 string      `json:"environment"`
	LicenseCostOnDemandPerHour  json.Number `json:"licenseCostOnDemandPerHour"`
	SubscriptionCostHourly      json.Number `json:"subscriptionCostHourly"`
	InstanceCostHourly          json.Number `json:"instanceCostHourly"`
	FreeTierHours               int64       `json:"freeTierHours"`
}

type CloudProviderRegions struct {
	DefaultRegionID   int64                   `json:"defaultRegionId"`
	DefaultInstanceID int64                   `json:"defaultInstanceId"`
	Regions           []CloudProviderRegion   `json:"regions"`
	Instances         []CloudProviderInstance `json:"instances"`
}

type CloudProviderInstances struct {
	DefaultInstanceID int64                   `json:"defaultInstanceId"`
	Instances         []CloudProviderInstance `json:"instances"`
}

type ClusterRequest struct {
	ID                  int64  `json:"id"`
	RequestType         string `json:"requestType"`
	AccountID           int64  `json:"accountID"`
	UserID              int64  `json:"userID"`
	Version             int64  `json:"version"`
	RequestBody         string `json:"requestBody"`
	ProgressPercent     int64  `json:"progressPercent"`
	ProgressDescription string `json:"progressDescription"`
	ClusterID           int64  `json:"clusterID"`
	Status              string `json:"status"`
}

type EncryptionAtRest struct {
	Provider string `json:"provider"`
}

type ClusterCreateRequest struct {
	AccountCredentialID      int64             `json:"accountCredentialId,omitempty"`
	AlternatorWriteIsolation string            `json:"alternatorWriteIsolation,omitempty"`
	BroadcastType            string            `json:"broadcastType,omitempty"`
	CidrBlock                string            `json:"cidrBlock,omitempty"`
	CloudProviderID          int64             `json:"cloudProviderId,omitempty"`
	InstanceID               int64             `json:"instanceId,omitempty"`
	RegionID                 int64             `json:"regionId,omitempty"`
	EnableDNSAssociation     bool              `json:"enableDnsAssociation"`
	AllowedIPs               []string          `json:"allowedIPs,omitempty"`
	FreeTier                 bool              `json:"freeTier"`
	JumpStart                bool              `json:"jumpStart"`
	ClusterName              string            `json:"clusterName"`
	NumberOfNodes            int64             `json:"numberOfNodes"`
	PromProxy                bool              `json:"promProxy"`
	ReplicationFactor        int64             `json:"replicationFactor"`
	ScyllaVersionID          int64             `json:"scyllaVersionId,omitempty"`
	UserAPIInterface         string            `json:"userApiInterface,omitempty"`
	Provisioning             string            `json:"provisioning,omitempty"`
	ProcessingUnits          int               `json:"pu,omitempty" minimum:"1" maximum:"1000" default:"1"`
	Expiration               string            `json:"expiration,omitempty" example:"12"`
	EncryptionAtRest         *EncryptionAtRest `json:"encryptionAtRest,omitempty"`
}

type Cluster struct {
	ID                  int64                  `json:"id"`
	AccountID           int64                  `json:"accountId"`
	ClusterName         string                 `json:"clusterName"`
	Status              string                 `json:"status"`
	InstanceID          int64                  `json:"instanceId"`
	CloudProviderID     int64                  `json:"cloudProviderID"`
	ScyllaVersionID     int64                  `json:"scyllaVersionID"`
	UserAPIInterface    string                 `json:"userApiInterface"`
	PricingModel        int64                  `json:"pricingModel"`
	MaxAllowedCIDRRange int64                  `json:"maxAllowedCidrRange"`
	DNS                 bool                   `json:"dns"`
	CloudProvider       *CloudProvider         `json:"cloudProvider"`
	ScyllaVersion       *ScyllaVersion         `json:"scyllaVersion"`
	Region              *CloudProviderRegion   `json:"region"`
	Instance            *CloudProviderInstance `json:"instance"`
	Datacenter          *Datacenter            `json:"dc"`
	FreeTier            *ExpirationTime        `json:"freeTier"`
	JumpStart           *ExpirationTime        `json:"jumpStart"`
	Progress            *Progress              `json:"progress"`

	ReplicationFactor        int64             `json:"replicationFactor,omitempty"`
	BroadcastType            string            `json:"broadcastType,omitempty"`
	GrafanaURL               string            `json:"grafanaUrl,omitempty"`
	ClientIP                 string            `json:"clientIp,omitempty"`
	CreatedAt                string            `json:"createdAt,omitempty"`
	PromProxyEnabled         bool              `json:"promProxyEnabled,omitempty"`
	AllowedIPs               []AllowedIP       `json:"allowedIps,omitempty"`
	Datacenters              []Datacenter      `json:"dataCenters,omitempty"`
	Nodes                    []Node            `json:"nodes,omitempty"`
	VPCList                  []VPC             `json:"vpcList,omitempty"`
	VPCPeeringList           []VPCPeering      `json:"vpcPeeringList,omitempty"`
	AlternatorWriteIsolation string            `json:"alternatorWriteIsolation,omitempty"`
	EncryptionAtRest         *EncryptionAtRest `json:"encryptionAtRest,omitempty"`
}

type Progress struct {
	ProgressPercent     int64  `json:"ProgressPercent"`
	ProgressDescription string `json:"ProgressDescription"`
}

type Clusters struct {
	Clusters []Cluster `json:"clusters"`
}

type ExpirationTime struct {
	ExpirationDate    string `json:"expirationDate"`
	ExpirationSeconds int64  `json:"expirationSeconds"`
	CreationTime      string `json:"creationTime"`
}

type Datacenter struct {
	ID                               int64                `json:"id"`
	Name                             string               `json:"Name"`
	Status                           string               `json:"Status"`
	ClusterID                        int64                `json:"ClusterID"`
	CloudProviderID                  int64                `json:"CloudProviderID"`
	RegionID                         int64                `json:"regionID"`
	InstanceID                       int64                `json:"instanceId"`
	ReplicationFactor                int64                `json:"ReplicationFactor"`
	CIDRBlock                        string               `json:"cidrBlock"`
	AccountCloudProviderCredentialID int64                `json:"accountCloudProviderCredentialsId"`
	CloudProvider                    *CloudProvider       `json:"cloudProvider,omitempty"`
	Region                           *CloudProviderRegion `json:"region,omitempty"`
}

type AllowedIP struct {
	ID        int64  `json:"id"`
	ClusterID int64  `json:"clusterId"`
	Address   string `json:"address"`
}

type Node struct {
	BillingStartDate string               `json:"billingStartDate"`
	CloudProviderID  int64                `json:"cloudProviderID"`
	InstanceID       int64                `json:"instanceId"`
	RegionID         int64                `json:"regionID"`
	DatacenterID     int64                `json:"dcID"`
	ClusterJoinDate  string               `json:"clusterJoinDate"`
	DNS              string               `json:"dns"`
	ID               int64                `json:"id"`
	State            string               `json:"state"`
	PrivateIP        string               `json:"privateIP"`
	PublicIP         string               `json:"publicIP"`
	CloudProvider    *CloudProvider       `json:"cloudProvider"`
	Region           *CloudProviderRegion `json:"region"`
	ServiceID        int64                `json:"serviceID"`
	ServiceVersionID int64                `json:"serviceVersionID"`
	Status           string               `json:"status"`
}

type AccountDeal struct {
	Type           string
	ProviderID     int
	IsEligible     bool
	ExpirationDate *time.Time `json:",omitempty" yaml:",omitempty"`
	StartDate      *time.Time `json:",omitempty" yaml:",omitempty"`
	Total          uint       `json:",omitempty" yaml:",omitempty"`
	Used           uint       `json:",omitempty" yaml:",omitempty"`
}

type VPC struct {
	ClusterID       int64  `json:"clusterID"`
	ID              int64  `json:"id"`
	CloudProviderID int64  `json:"cloudProviderId"`
	RegionID        int64  `json:"regionId"`
	CIDRBlock       string `json:"cidrBlock"`
}

type VPCPeeringRequest struct {
	DatacenterID int64  `json:"dcId"`
	AllowCQL     bool   `json:"allowCql"`
	VPC          string `json:"vpcId"`
	CidrBlock    string `json:"cidrBlock"`
	Owner        string `json:"ownerId"`
	RegionID     int64  `json:"regionId"`
}

type VPCPeering struct {
	ExternalID       string   `json:"externalId"`
	ID               int64    `json:"id"`
	NetworkName      string   `json:"networkName"`
	OwnerID          string   `json:"ownerId"`
	VPCID            string   `json:"vpcId"`
	CIDRList         []string `json:"cidrList"`
	CIDRListVerified []string `json:"cidrBlockVerified"`
	RegionID         int64    `json:"regionId"`
	ProjectID        string   `json:"projectID"`
	Status           string   `json:"status"`
	ExpiresAt        string   `json:"expiresAt"`
	AllowCQL         bool     `json:"allowCql"`
}

func (vp *VPCPeering) NetworkLink() string {
	return "projects/" + vp.ProjectID + "/global/networks/" + vp.NetworkName
}

type ClusterConnectionCreateRequest struct {
	Name        string            `json:"name"`
	CIDRList    []string          `json:"cidrList"`
	ClusterDCID int64             `json:"clusterDCID"`
	Data        map[string]string `json:"data"`
	Type        string            `json:"type"`
}

type ClusterConnectionUpdateRequest struct {
	Name     string   `json:"name"`
	CIDRList []string `json:"cidrList"`
	Status   string   `json:"status"`
}

type ClusterConnection struct {
	ID                int64             `json:"id"`
	Name              string            `json:"name"`
	ExternalID        string            `json:"externalId"`
	AwaitingData      map[string]string `json:"awaitingData"`
	AwaitingForClient bool              `json:"awaitingForClient"`
	CIDRList          []string          `json:"cidrList"`
	ClusterDCID       int64             `json:"clusterDCID"`
	ClusterVPCID      int64             `json:"clusterVPCID"`
	ClusterID         int64             `json:"clusterId"`
	Data              map[string]string `json:"data"`
	Stage             string            `json:"stage"`
	StageMessage      string            `json:"stageMessage"`
	Status            string            `json:"status"`
	Type              string            `json:"type"`
}

type ClusterConnectionInformation struct {
	BroadcastType string `json:"broadcastType"`
	Credentials   struct {
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"credentials"`
	Datacenters []DatacenterConnection `json:"connectDataCenters"`
}

type DatacenterConnection struct {
	Name      string   `json:"dcName"`
	PublicIP  []string `json:"publicIPs"`
	PrivateIP []string `json:"privateIPs"`
	DNS       []string `json:"dns"`
}

type ClusterDetails struct {
	Cluster Cluster `json:"cluster"`
}

type Nodes struct {
	Nodes []Node `json:"nodes"`
}

func NodesByStatus(n []Node, status string) (f []Node) {
	for i := range n {
		if strings.EqualFold(n[i].Status, status) {
			f = append(f, n[i])
		}
	}
	return f
}

func NodesPrivateIPs(n []Node) []string {
	ips := make([]string, 0, len(n))
	for i := range n {
		ips = append(ips, n[i].PrivateIP)
	}
	return ips
}

func NodesDNSNames(n []Node) []string {
	dns := make([]string, 0, len(n))
	for i := range n {
		dns = append(dns, n[i].DNS)
	}
	return dns
}

type Datacenters struct {
	Datacenters []Datacenter `json:"dataCenters"`
}

type Stack struct {
	RequestType        string         `json:"RequestType"`
	RequestID          string         `json:"RequestId"`
	ResourceProperties map[string]any `json:"ResourceProperties"`
}
