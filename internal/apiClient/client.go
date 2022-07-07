// Package apiClient is a wrapper for the Scylla Cloud REST API.
// TODO if sufficiently high quality it can be published as a separate SDK in the future.
package apiClient

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"
)

var DefaultTimeout = 60 * time.Second

const DefaultEndpoint = "https://cloud.scylladb.com/api/v0"

// Client represents a client to call the Scylla Cloud API
type Client struct {
	// token holds the bearer token used for authentication.
	token string

	// accountId holds the account ID used in requests to the API.
	accountId int64

	// API endpoint
	endpoint string

	// Client is the underlying HTTP client used to run the requests.
	Client *http.Client

	// Ensures that the timeDelta function is only ran once
	// sync.Once would consider init done, even in case of error
	// hence a good old flag
	timeDeltaMutex *sync.Mutex
	timeDeltaDone  bool
	timeDelta      time.Duration
	Timeout        time.Duration
}

// NewClient represents a new client to call the API
func NewClient(endpoint, token string) (*Client, error) {
	client := Client{
		token:          token,
		Client:         &http.Client{},
		timeDeltaMutex: &sync.Mutex{},
		timeDeltaDone:  false,
		Timeout:        time.Duration(DefaultTimeout),
		endpoint:       endpoint,
	}

	if err := client.findAccountId(); err != nil {
		return nil, err
	}

	return &client, nil
}

type UserAccount struct {
	UserId            int64  `json:"UserID"`
	AccountId         int64  `json:"AccountID"`
	Name              string `json:"Name"`
	OwnerUserId       int64  `json:"OwnerUserID"`
	AccountStatus     string `json:"AccountStatus"`
	Role              string `json:"Role"`
	UserAccountStatus string `json:"UserAccountStatus"`
}

func (c *Client) findAccountId() error {
	var result UserAccount
	if err := c.Get("/account/default", &result); err != nil {
		return err
	}

	c.accountId = result.AccountId
	return nil
}

// Don't review it, it'll be overhauled later.
func (c *Client) Get(path string, resultType interface{}) error {
	url := c.endpoint + path

	httpClient := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)

	if err != nil {
		return err
	}

	req.Header.Add("accept", "application/json")
	req.Header.Add("Authorization", "Bearer "+c.token)

	res, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		//apiError := &APIError{Code: response.StatusCode}
		//if err = json.Unmarshal(body, apiError); err != nil {
		//	apiError.Message = string(body)
		//}

		return errors.New(fmt.Sprintf("HTTP request to '%s' failed with code %d: %s", url, res.StatusCode, string(body)))
	}

	d := json.NewDecoder(bytes.NewReader(body))
	d.UseNumber()
	if err := d.Decode(resultType); err != nil {
		return err
	}
	return nil
}

type CloudProvider struct {
	Id            int64  `json:"ID"`
	Name          string `json:"Name"`
	RootAccountId string `json:"RootAccountID"`
}

func (c *Client) ListCloudProviders() ([]CloudProvider, error) {
	var result []CloudProvider
	if err := c.Get("/deployment/provider", &result); err != nil {
		return nil, err
	}
	return result, nil
}

type CloudProviderRegion struct {
	Id                          int64  `json:"ID"`
	CloudProviderId             int64  `json:"CloudProviderID"`
	Name                        string `json:"Name"`
	FullName                    string `json:"FullName"`
	ExternalId                  string `json:"ExternalID"`
	MultiRegionExternalId       string `json:"MultiRegionExternalID"`
	DcName                      string `json:"DCName"`
	BackupStorageGbCost         string `json:"BackupStorageGBCost"`
	TrafficSameRegionInGbCost   string `json:"TrafficSameRegionInGBCost"`
	TrafficSameRegionOutGbCost  string `json:"TrafficSameRegionOutGBCost"`
	TrafficCrossRegionOutGbCost string `json:"TrafficCrossRegionOutGBCost"`
	TrafficInternetOutGbCost    string `json:"TrafficInternetOutGBCost"`
	Continent                   string `json:"Continent"`
}

func (c *Client) ListCloudProviderRegions(providerId int64) ([]CloudProviderRegion, error) {
	var result []CloudProviderRegion
	path := fmt.Sprintf("/deployment/provider/%d/region", providerId)
	if err := c.Get(path, &result); err != nil {
		return nil, err
	}
	return result, nil
}
