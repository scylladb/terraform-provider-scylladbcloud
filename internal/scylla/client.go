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
	maxRetry                    = 10
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

func parseRetryAfter(val string) time.Duration {
	if n, err := strconv.ParseUint(val, 10, 64); err == nil {
		return time.Duration(n) * time.Second
	}
	if t, err := time.Parse(time.RFC1123, val); err == nil {
		return time.Until(t)
	}
	return 0
}

func (c *Client) callAPIonce(ctx context.Context, method, path string, reqBody, resType interface{}, query ...string) (err error) {
	traceData := map[string]interface{}{
		"method": method,
		"path":   path,
	}

	defer func() {
		if err == nil {
			tflog.Trace(ctx, "api call succeeded", traceData)
			return
		}
		tflog.Trace(ctx, "api call failed:"+err.Error(), traceData)
	}()

	ctx = tfcontext.AddHttpRequestInfo(ctx, method, path)
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

	rqURL := resp.Request.URL.String()
	statusCode := resp.StatusCode
	status := resp.Status
	retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))

	var buf bytes.Buffer
	var body = io.TeeReader(io.LimitReader(resp.Body, maxResponseBodyLength), &buf)

	traceData["status"] = buf.String()
	traceData["code"] = statusCode
	traceData["status"] = status

	if p, ok := resType.(*[]byte); ok {
		if *p, err = io.ReadAll(body); err != nil {
			return makeAPIError("error reading body: "+err.Error(), c.ErrCodes, rqURL, method, statusCode, retryAfter)
		}
		return nil
	}

	d := json.NewDecoder(body)
	d.UseNumber()

	var data = struct {
		Error string      `json:"error"`
		Data  interface{} `json:"data"`
	}{"", resType}

	if err = d.Decode(&data); err != nil {
		return makeAPIError("failed to unmarshal data: "+err.Error(), c.ErrCodes, rqURL, method, statusCode, retryAfter)
	}

	if statusCode < 200 || statusCode >= 300 {
		if data.Error == "" {
			data.Error = http.StatusText(statusCode)
		}
	}

	if data.Error != "" {
		return makeAPIError(data.Error, c.ErrCodes, rqURL, method, statusCode, retryAfter)
	}

	return nil
}

func (c *Client) callAPI(ctx context.Context, method, path string, reqBody, resType interface{}, query ...string) (err error) {
	for retry := 0; retry <= maxRetry; retry++ {
		err = c.callAPIonce(ctx, method, path, reqBody, resType, query...)
		if err == nil {
			return nil
		}
		tflog.Trace(ctx, fmt.Sprintf("ERRRRRRR %T %+v", err, err))
		isRetryable, timeToSleep := getRetryInfo(ctx, err)
		tflog.Trace(ctx, fmt.Sprintf("isRetryable=%t, timeToSleep=%s", isRetryable, timeToSleep))
		if !isRetryable {
			return err
		}
		if timeToSleep == 0 {
			timeToSleep = 100 * time.Millisecond
		}
		if err = sleepUntilCanceled(ctx, timeToSleep); err != nil {
			return err
		}
	}
	return err
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

func getRetryInfo(ctx context.Context, err error) (retryable bool, retryAfter time.Duration) {
	if oe, ok := err.(*net.OpError); ok {
		return oe.Temporary(), 0
	}

	if apiErr := castToAPIError(err); apiErr != nil {
		tflog.Trace(ctx, fmt.Sprintf("APIError code=%q, statuscode=%d", apiErr.Code, apiErr.StatusCode))
		return apiErr.Temporary(), apiErr.RetryAfter
	}
	return false, 0
}

func sleepUntilCanceled(ctx context.Context, timeToSleep time.Duration) error {
	timer := time.NewTimer(timeToSleep)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func init() {
	var err error
	errCodes, err = parse(codes, codesDelim, codesFunc)
	if err != nil {
		panic(fmt.Errorf("failed to parse error codes: %w", err))
	}
}
