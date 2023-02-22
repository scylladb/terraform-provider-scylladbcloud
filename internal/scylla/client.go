package scylla

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	stdpath "path"

	rhttp "github.com/hashicorp/go-retryablehttp"
)

var (
	retriesAllowed              = 3
	maxResponseBodyLength int64 = 1 << 20
)

// Client represents a client to call the Scylla Cloud API
type Client struct {
	Meta *Cloudmeta
	// headers holds headers that will be set for all http requests.
	Headers http.Header
	// AccountID holds the account ID used in requests to the API.
	AccountID int64
	// API endpoint
	Endpoint *url.URL
	// HTTPClient is the underlying HTTP client used to run the requests.
	// It may be overloaded but a default one is provided in ``NewClient`` by default.
	HTTPClient *rhttp.Client
}

// NewClient represents a new client to call the API
func (c *Client) Auth(ctx context.Context, token string) error {
	if c.HTTPClient == nil {
		c.HTTPClient = rhttp.NewClient()
	}

	if c.Headers == nil {
		c.Headers = make(http.Header)
	}

	c.Headers.Set("Authorization", "Bearer "+token)

	if c.Meta == nil {
		var err error
		if c.Meta, err = BuildCloudmeta(ctx, c); err != nil {
			return fmt.Errorf("error building metadata: %w", err)
		}
	}

	if err := c.findAndSaveAccountID(ctx); err != nil {
		return err
	}

	return nil
}

func (c *Client) newHttpRequest(ctx context.Context, method, path string, reqBody interface{}, query ...string) (*rhttp.Request, error) {
	var body []byte
	var err error

	if reqBody != nil {
		body, err = json.Marshal(reqBody)
		if err != nil {
			return nil, err
		}

		fmt.Printf("[DEBUG] %s %s body:\n%s\n", method, path, body)
	}

	url := *c.Endpoint
	url.Path = stdpath.Join("/", url.Path, path)

	req, err := rhttp.NewRequestWithContext(ctx, method, url.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header = c.Headers
	if body != nil {
		req.Header = req.Header.Clone()
		req.Header.Add("Content-Type", "application/json;charset=utf-8")
	}

	if len(query) != 0 {
		if len(query)%2 != 0 {
			return nil, errors.New("odd number of query arguments")
		}

		for i := 0; i < len(query); i += 2 {
			q := req.URL.Query()
			q.Set(query[i], query[i+1])
			req.URL.RawQuery = q.Encode()
		}
	}

	return req, nil
}

func (c *Client) callAPI(ctx context.Context, method, path string, reqBody, resType interface{}, query ...string) error {
	req, err := c.newHttpRequest(ctx, method, path, reqBody, query...)
	if err != nil {
		return err
	}
	req = req.WithContext(ctx)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var buf bytes.Buffer

	fmt.Fprintf(&buf, "[DEBUG] (%d) %s %s:\n\n", resp.StatusCode, req.Method, req.URL)
	defer func() {
		fmt.Printf("%s\n\n", &buf)
	}()

	var body io.Reader = io.TeeReader(io.LimitReader(resp.Body, maxResponseBodyLength), &buf)

	if p, ok := resType.(*[]byte); ok {
		if *p, err = io.ReadAll(body); err != nil {
			return fmt.Errorf("error reading body: %w", err)
		}

		return nil
	}

	d := json.NewDecoder(body)
	d.UseNumber()

	var data = struct {
		Error string      `json:"error"`
		Data  interface{} `json:"data"`
	}{"", resType}

	if err := d.Decode(&data); err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if data.Error == "" {
			data.Error = http.StatusText(resp.StatusCode)
		}
	}

	if data.Error != "" {
		return makeError(data.Error, c.Meta.ErrCodes, resp)
	}

	return nil
}

func (c *Client) get(ctx context.Context, path string, resultType interface{}, query ...string) error {
	return c.callAPI(ctx, http.MethodGet, path, nil, resultType, query...)
}

func (c *Client) post(ctx context.Context, path string, requestBody, resultType interface{}) error {
	return c.callAPI(ctx, http.MethodPost, path, requestBody, resultType)
}

func (c *Client) delete(ctx context.Context, path string) error {
	return c.callAPI(ctx, http.MethodDelete, path, nil, nil)
}

func (c *Client) findAndSaveAccountID(ctx context.Context) error {
	var result struct {
		AccountID int64 `json:"accountId"`
	}

	if err := c.get(ctx, "/account/default", &result); err != nil {
		return err
	}

	c.AccountID = result.AccountID

	return nil
}
