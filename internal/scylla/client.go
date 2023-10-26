package scylla

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	stdpath "path"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/tfcontext"
)

const (
	defaultTimeout              = 60 * time.Second
	retriesAllowed              = 3
	maxResponseBodyLength int64 = 1 << 20
)

// Client represents a client to call the Scylla Cloud API
type Client struct {
	Meta *Cloudmeta
	// headers holds headers that will be set for all http requests.
	Headers http.Header
	// API endpoint
	Endpoint *url.URL
	// ErrCodes provides a human-readable translation of ScyllaDB API errors
	ErrCodes map[string]string // code -> message
	// HTTPClient is the underlying HTTP client used to run the requests.
	// It may be overloaded but a default one is provided in ``NewClient`` by default.
	HTTPClient *http.Client
	// AccountID holds the account ID used in requests to the API.
	AccountID int64
}

func NewClient() (*Client, error) {
	errCodes, err := parse(codes, codesDelim, codesFunc)
	if err != nil {
		return nil, fmt.Errorf("failed to parse error codes: %w", err)
	}

	c := &Client{
		ErrCodes:   errCodes,
		Headers:    make(http.Header),
		HTTPClient: http.DefaultClient,
	}

	return c, nil
}

// NewClient represents a new client to call the API
func (c *Client) Auth(ctx context.Context, token string) error {
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: defaultTimeout}
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

func (c *Client) newHttpRequest(ctx context.Context, method, path string, reqBody interface{}, query ...string) (*http.Request, error) {
	var body []byte
	var err error

	if reqBody != nil {
		body, err = json.Marshal(reqBody)
		if err != nil {
			return nil, err
		}
	}

	url := *c.Endpoint
	url.Path = stdpath.Join("/", url.Path, path)

	req, err := http.NewRequestWithContext(ctx, method, url.String(), bytes.NewReader(body))
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
	tflog.Trace(ctx, "api call prepared: "+req.Method+" "+req.URL.String(), map[string]interface{}{
		"host":        req.Host,
		"remote_addr": req.RemoteAddr,
		"body":        string(body),
	})

	return req, nil
}

func (c *Client) doHttpRequest(req *http.Request) (resp *http.Response, temporaryErr bool, err error) {
	resp, err = c.HTTPClient.Do(req)
	if err != nil {
		if oe, ok := err.(*net.OpError); ok {
			temporaryErr = oe.Temporary()
		}

		return
	}

	temporaryErr = resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusGatewayTimeout || resp.StatusCode == http.StatusTooManyRequests
	return
}

func (c *Client) doHttpRequestWithRetries(req *http.Request, retries int, retryBackoffDuration time.Duration) (*http.Response, error) {
	resp, temporaryErr, err := c.doHttpRequest(req)
	if temporaryErr && retries > 0 {
		if err == nil {
			_ = resp.Body.Close() // We want to retry anyway.
		}
		var timeToSleep time.Duration
		if d, ok := parseRetryAfter(resp.Header.Get("Retry-After")); ok {
			timeToSleep = d
		} else {
			timeToSleep = retryBackoffDuration
		}
		timer := time.NewTimer(timeToSleep)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}

		return c.doHttpRequestWithRetries(req, retries-1, retryBackoffDuration*2)
	}

	return resp, err
}

func parseRetryAfter(val string) (time.Duration, bool) {
	if n, err := strconv.ParseUint(val, 10, 64); err == nil {
		return time.Duration(n) * time.Second, true
	}
	if t, err := time.Parse(time.RFC1123, val); err == nil {
		return time.Until(t), true
	}
	return 0, false
}

func (c *Client) callAPI(ctx context.Context, method, path string, reqBody, resType interface{}, query ...string) error {
	ctx = tfcontext.AddHttpRequestInfo(ctx, method, path)
	req, err := c.newHttpRequest(ctx, method, path, reqBody, query...)
	if err != nil {
		return err
	}
	req = req.WithContext(ctx)

	resp, err := c.doHttpRequestWithRetries(req, retriesAllowed, time.Second)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var buf bytes.Buffer

	var body io.Reader = io.TeeReader(io.LimitReader(resp.Body, maxResponseBodyLength), &buf)

	if p, ok := resType.(*[]byte); ok {
		if *p, err = io.ReadAll(body); err != nil {
			tflog.Trace(ctx, "failed to read body: "+err.Error(), map[string]interface{}{
				"code":   resp.StatusCode,
				"status": resp.Status,
				"error":  err.Error(),
			})
			return fmt.Errorf("error reading body: %w", err)
		}
		tflog.Trace(ctx, "api call succeeded", map[string]interface{}{
			"code":   resp.StatusCode,
			"status": resp.Status,
			"body":   buf.String(),
		})
		return nil
	}

	d := json.NewDecoder(body)
	d.UseNumber()

	var data = struct {
		Error string      `json:"error"`
		Data  interface{} `json:"data"`
	}{"", resType}

	if err := d.Decode(&data); err != nil {
		tflog.Trace(ctx, "failed to unmarshal data: "+err.Error(), map[string]interface{}{
			"code":   resp.StatusCode,
			"status": resp.Status,
			"error":  err.Error(),
			"body":   buf.String(),
		})
		err = makeError("failed to unmarshal data: "+err.Error(), c.ErrCodes, resp)
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if data.Error == "" {
			data.Error = http.StatusText(resp.StatusCode)
		}
	}

	if data.Error != "" {
		err = makeError(data.Error, c.ErrCodes, resp)
		tflog.Trace(ctx, "api returned error: "+err.Error(), map[string]interface{}{
			"code":   resp.StatusCode,
			"status": resp.Status,
			"body":   buf.String(),
			"error":  err.Error(),
		})
		return err
	}

	tflog.Trace(ctx, "api call succeeded", map[string]interface{}{
		"code":   resp.StatusCode,
		"status": resp.Status,
		"body":   buf.String(),
	})
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
