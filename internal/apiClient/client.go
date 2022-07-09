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

	// httpClient is the underlying HTTP client used to run the requests.
	httpClient *http.Client
}

// NewClient represents a new client to call the API
func NewClient(endpoint, token string) (*Client, error) {
	client := Client{
		token: token,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
		endpoint: endpoint,
	}

	if err := client.findAndSaveAccountId(); err != nil {
		return nil, err
	}

	return &client, nil
}

func (c *Client) get(path string, resultType interface{}) error {
	url := c.endpoint + path

	req, err := http.NewRequest(http.MethodGet, url, nil)

	if err != nil {
		return err
	}

	req.Header.Add("accept", "application/json")
	req.Header.Add("Authorization", "Bearer "+c.token)

	res, err := c.httpClient.Do(req)
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

func (c *Client) findAndSaveAccountId() error {
	var result UserAccount
	if err := c.get("/account/default", &result); err != nil {
		return err
	}

	c.accountId = result.AccountId
	return nil
}

func (c *Client) ListCloudProviders() ([]CloudProvider, error) {
	var result []CloudProvider
	if err := c.get("/deployment/provider", &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) ListCloudProviderRegions(providerId int64) ([]CloudProviderRegion, error) {
	var result []CloudProviderRegion
	path := fmt.Sprintf("/deployment/provider/%d/region", providerId)
	if err := c.get(path, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) ListClusters() ([]Cluster, error) {
	type Item struct {
		Value Cluster     `json:"Value"`
		Error interface{} `json:"Error"`
	}
	var result []Item
	path := fmt.Sprintf("/account/%d/cluster", c.accountId)
	if err := c.get(path, &result); err != nil {
		return nil, err
	}

	clusters := make([]Cluster, len(result))
	for i, item := range result {
		if item.Error != nil {
			return nil, errors.New(fmt.Sprintf("cluster error: %v", item.Error))
		}
		clusters[i] = item.Value
	}
	return clusters, nil
}
