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
	// Token holds the bearer token used for authentication.
	Token string

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
func NewClient(endpoint, token string, accountId int64) (*Client, error) {
	client := Client{
		Token:          token,
		Client:         &http.Client{},
		timeDeltaMutex: &sync.Mutex{},
		timeDeltaDone:  false,
		Timeout:        time.Duration(DefaultTimeout),
		accountId:      accountId,
		endpoint:       endpoint,
	}

	return &client, nil
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
	req.Header.Add("Authorization", "Bearer "+c.Token)

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
