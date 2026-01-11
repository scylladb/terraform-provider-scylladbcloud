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
	"time"

	v2scylla "github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/v2"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/tfcontext"

	"github.com/eapache/go-resiliency/retrier"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	defaultTimeout              = 60 * time.Second
	retriesAllowed              = 3
	maxResponseBodyLength int64 = 1 << 20
)

// Client represents a client to call the Scylla Cloud API
type Client struct {
	Meta *Cloudmeta

	// Headers holds headers that will be set for all http requests.
	Headers http.Header

	// API endpoint
	Endpoint *url.URL

	// Token is the API token used to authenticate requests.
	Token string

	// ErrCodes provides a human-readable translation of ScyllaDB API errors.
	ErrCodes map[string]string // code -> message

	// HTTPClient is the underlying HTTP client used to run the requests.
	// It may be overloaded but a default one is provided in ``NewClient`` by default.
	HTTPClient *http.Client

	// AccountID holds the account ID used in requests to the API.
	AccountID int64

	// Retry is used to retry requests to the API.
	Retry *retrier.Retrier

	// V2 is the client to call the V2 API, it does not require costly
	// metadata building.
	V2 *v2scylla.Client
}

func NewClient(endpoint, token, useragent string, metadata bool) (*Client, error) {
	errCodes, err := parse(codes, codesDelim, codesFunc)
	if err != nil {
		return nil, fmt.Errorf("failed to parse error codes: %w", err)
	}

	end, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	retry := retrier.New(
		retrier.ExponentialBackoff(5, 5*time.Second),
		DefaultClassifier,
	)

	c := &Client{
		Token:      token,
		ErrCodes:   errCodes,
		Headers:    make(http.Header),
		HTTPClient: &http.Client{Timeout: defaultTimeout},
		Retry:      retry,
		Endpoint:   end,
		V2: v2scylla.New(
			v2scylla.WithRetryPolicy(retry),
			v2scylla.WithUserAgent(useragent),
			v2scylla.WithBaseURL(endpoint),
			v2scylla.WithGlobalCookieJar(),
		),
	}

	c.Headers.Set("Authorization", "Bearer "+c.Token)
	c.Headers.Set("Accept", "application/json; charset=utf-8")
	c.Headers.Set("User-Agent", useragent)

	if metadata {
		if c.Meta, err = BuildCloudmeta(ctx, c); err != nil {
			return nil, fmt.Errorf("error building metadata: %w", err)
		}
		if err = c.findAndSaveAccountID(ctx); err != nil {
			return nil, err
		}
	}

	return c, nil
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

	req.Header = c.Headers.Clone()
	if body != nil {
		req.Header.Add("Content-Type", "application/json;charset=utf-8")
	}

	if len(query) != 0 {
		if len(query)%2 != 0 {
			return nil, errors.New("odd number of query arguments")
		}

		q := req.URL.Query()
		for i := 0; i < len(query); i += 2 {
			q.Set(query[i], query[i+1])
		}
		req.URL.RawQuery = q.Encode()
	}

	tflog.Trace(ctx, "api call prepared: "+req.Method+" "+req.URL.String(), map[string]interface{}{
		"host":        req.Host,
		"remote_addr": req.RemoteAddr,
		"body":        string(body),
	})

	return req, nil
}

func (c *Client) retryCall(ctx context.Context, method, path string, reqBody, resType interface{}, query ...string) error {
	return c.Retry.RunCtx(ctx, func(ctx context.Context) error {
		return c.call(ctx, method, path, reqBody, resType, query...)
	})
}

func (c *Client) call(ctx context.Context, method, path string, reqBody, resType interface{}, query ...string) error {
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
	defer func() { _ = resp.Body.Close() }()

	var (
		buf  bytes.Buffer
		body = io.TeeReader(
			io.LimitReader(resp.Body, maxResponseBodyLength),
			&buf,
		)
	)

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

	data := struct {
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
	return c.retryCall(ctx, http.MethodGet, path, nil, resultType, query...)
}

func (c *Client) post(ctx context.Context, path string, requestBody, resultType interface{}) error {
	return c.retryCall(ctx, http.MethodPost, path, requestBody, resultType)
}

func (c *Client) patch(ctx context.Context, path string, requestBody, resultType interface{}) error {
	return c.retryCall(ctx, http.MethodPatch, path, requestBody, resultType)
}

func (c *Client) delete(ctx context.Context, path string) error {
	return c.retryCall(ctx, http.MethodDelete, path, nil, nil)
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
