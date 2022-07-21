// Package scylla is a wrapper for the Scylla Cloud REST API.
package scylla

// TODO if sufficiently high quality it can be published as a separate SDK in the future.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/scylladb/terraform-provider-scylla/internal/scylla/model"
	"io"
	"net"
	"net/http"
	"time"
)

var DefaultTimeout = 60 * time.Second

const DefaultEndpoint = "https://cloud.scylladb.com/api/v0"

var retriesAllowed = 3
var maxResponseBodyLength int64 = 1 << 20

// APIError represents an error that occurred while calling the API.
type APIError struct {
	// API error code (meanings are described in the Scylla Cloud API documentation)
	Code string `json:"code"`
	// Error message.
	Message string `json:"message"`
	// Error details
	TraceID string `json:"trace_id"`
	// Http status code
	StatusCode int
}

func (err *APIError) Error() string {
	return fmt.Sprintf(
		"Error %q (http status %d): %q. Trace id: %q.",
		err.Code, err.StatusCode, err.Message, err.TraceID)
}

// Client represents a client to call the Scylla Cloud API
type Client struct {
	// headers holds headers that will be set for all http requests.
	headers http.Header

	// accountID holds the account ID used in requests to the API.
	accountID int64

	// API endpoint
	endpoint string

	// HTTPClient is the underlying HTTP client used to run the requests.
	// It may be overloaded but a default one is provided in ``NewClient`` by default.
	HTTPClient *http.Client
}

// NewClient represents a new client to call the API
func NewClient(endpoint, token string, httpClient *http.Client) (*Client, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: DefaultTimeout}
	}

	client := Client{
		headers:    http.Header{},
		endpoint:   endpoint,
		HTTPClient: httpClient,
	}

	client.headers.Add("Authorization", "Bearer "+token)
	client.headers.Add("Accept", "application/json")

	if err := client.findAndSaveAccountID(); err != nil {
		return nil, err
	}

	return &client, nil
}

func (c *Client) newHttpRequest(method, path string, reqBody interface{}) (*http.Request, error) {
	var body []byte
	var err error

	if reqBody != nil {
		body, err = json.Marshal(reqBody)
		if err != nil {
			return nil, err
		}
	}

	url := c.endpoint + path
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header = c.headers
	if body != nil {
		req.Header = req.Header.Clone()
		req.Header.Add("Content-Type", "application/json;charset=utf-8")
	}

	return req, nil
}

func (c *Client) doHttpRequest(req *http.Request) (resp *http.Response, temporaryErr bool, err error) {
	resp, err = c.HTTPClient.Do(req)
	if err != nil {
		temporaryErr = err.(*net.OpError).Temporary()
		return
	}

	temporaryErr = resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusGatewayTimeout
	return
}

func (c *Client) doHttpRequestWithRetries(req *http.Request, retries int, retryBackoffDuration time.Duration) (*http.Response, error) {
	resp, temporaryErr, err := c.doHttpRequest(req)
	if temporaryErr && retries > 0 {
		if err == nil {
			_ = resp.Body.Close() // We want to retry anyway.
		}
		return c.doHttpRequestWithRetries(req, retries-1, retryBackoffDuration*2)
	}

	return resp, err
}

func (c *Client) callAPI(ctx context.Context, method, path string, reqBody, resType interface{}) error {
	req, err := c.newHttpRequest(method, path, reqBody)
	if err != nil {
		return err
	}
	req = req.WithContext(ctx)

	resp, err := c.doHttpRequestWithRetries(req, retriesAllowed, time.Second)
	if err != nil {
		return err
	}

	d := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBodyLength))
	d.UseNumber()

	if resp.StatusCode < 200 || resp.StatusCode > 300 {
		type ErrorResponse struct {
			Error APIError `json:"Error"`
		}
		errorResponse := ErrorResponse{APIError{StatusCode: resp.StatusCode}}
		if err = d.Decode(&errorResponse); err != nil {
			return &APIError{StatusCode: resp.StatusCode}
		}
		return &errorResponse.Error
	}

	if resType == nil {
		return nil
	}

	return d.Decode(&resType)
}

func (c *Client) get(path string, resultType interface{}) error {
	return c.callAPI(context.TODO(), http.MethodGet, path, nil, resultType)
}

func (c *Client) post(path string, requestBody, resultType interface{}) error {
	return c.callAPI(context.TODO(), http.MethodPost, path, requestBody, resultType)
}

func (c *Client) delete(path string) error {
	return c.callAPI(context.TODO(), http.MethodDelete, path, nil, nil)
}

func (c *Client) findAndSaveAccountID() error {
	var result model.UserAccount
	if err := c.get("/account/default", &result); err != nil {
		return err
	}

	c.accountID = result.AccountID
	return nil
}
