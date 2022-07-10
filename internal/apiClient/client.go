// Package apiClient is a wrapper for the Scylla Cloud REST API.
// TODO if sufficiently high quality it can be published as a separate SDK in the future.
package apiClient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

var DefaultTimeout = 60 * time.Second

const DefaultEndpoint = "https://cloud.scylladb.com/api/v0"

// APIError represents an error that occurred while calling the API.
type APIError struct {
	// API error code (meanings are described in the Scylla Cloud API documentation)
	Code string `json:"code"`
	// Error message.
	Message string `json:"message"`
	// Error details
	TraceId string `json:"trace_id"`
	// Http status code
	StatusCode int
}

func (err *APIError) Error() string {
	return fmt.Sprintf(
		"Error %q (http status %d): %q. Trace id: %q.",
		err.Code, err.StatusCode, err.Message, err.TraceId)
}

// Client represents a client to call the Scylla Cloud API
type Client struct {
	// token holds the bearer token used for authentication.
	token string

	// accountId holds the account ID used in requests to the API.
	accountId int64

	// API endpoint
	endpoint string

	// HttpClient is the underlying HTTP client used to run the requests.
	// It may be overloaded but a default one is provided in ``NewClient`` by default.
	HttpClient *http.Client
}

// NewClient represents a new client to call the API
func NewClient(endpoint, token string, httpClient *http.Client) (*Client, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: DefaultTimeout}
	}
	client := Client{
		token:      token,
		endpoint:   endpoint,
		HttpClient: httpClient,
	}

	if err := client.findAndSaveAccountId(); err != nil {
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

	req.Header.Add("Authorization", "Bearer "+c.token)
	req.Header.Add("Accept", "application/json")
	if body != nil {
		req.Header.Add("Content-Type", "application/json;charset=utf-8")
	}

	return req, nil
}

func (c *Client) doHttpRequest(req *http.Request) (*http.Response, []byte, error) {
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return resp, nil, err
	}

	return resp, body, nil
}

func (c *Client) callAPI(ctx context.Context, method, path string, reqBody, resType interface{}) error {
	httpRequest, err := c.newHttpRequest(method, path, reqBody)
	httpRequest = httpRequest.WithContext(ctx)

	resp, body, err := c.doHttpRequest(httpRequest)
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		type ErrorResponse struct {
			Error APIError `json:"Error"`
		}
		errorResponse := ErrorResponse{APIError{StatusCode: resp.StatusCode}}
		if err = json.Unmarshal(body, &errorResponse); err != nil {
			return &APIError{StatusCode: resp.StatusCode, Message: string(body)}
		}

		return &errorResponse.Error
	}

	return c.unmarshalResponse(body, resType)
}

func (c *Client) unmarshalResponse(body []byte, resType interface{}) error {
	// Nothing to unmarshal
	if len(body) == 0 || resType == nil {
		return nil
	}

	d := json.NewDecoder(bytes.NewReader(body))
	d.UseNumber()
	return d.Decode(&resType)
}

func (c *Client) get(path string, resultType interface{}) error {
	return c.callAPI(context.Background(), http.MethodGet, path, nil, resultType)
}

func (c *Client) post(path string, requestBody, resultType interface{}) error {
	return c.callAPI(context.Background(), http.MethodGet, path, requestBody, resultType)
}

func (c *Client) delete(path string, resultType interface{}) error {
	return c.callAPI(context.Background(), http.MethodGet, path, nil, resultType)
}

func (c *Client) findAndSaveAccountId() error {
	var result UserAccount
	if err := c.get("/account/default", &result); err != nil {
		return err
	}

	c.accountId = result.AccountId
	return nil
}
