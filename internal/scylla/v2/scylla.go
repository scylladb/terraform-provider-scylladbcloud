package scylla

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/eapache/go-resiliency/retrier"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

type Client struct {
	reqmw  []func(*http.Request)
	client *http.Client
	retry  *retrier.Retrier
}

func New(opts ...func(*Client)) *Client {
	return (&Client{
		client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}).With(opts...)
}

func (c *Client) With(opts ...func(*Client)) *Client {
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Client) Request(ctx context.Context, method string, payload interface{}, format string, args ...interface{}) *http.Request {
	var body io.Reader
	if payload != nil {
		p, err := json.Marshal(payload)
		if err != nil {
			panic("unexpected error marshaling payload: " + err.Error())
		}
		body = bytes.NewReader(p)
	}

	req, err := http.NewRequestWithContext(ctx, method, buildURL(format, args...), body)
	if err != nil {
		panic("unexpected error creating request: " + err.Error())
	}

	for _, mw := range c.reqmw {
		mw(req)
	}

	req.Header.Set("Accept", "application/json")

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return req
}

func (c *Client) Do(req *http.Request, out interface{}) (*http.Response, error) {
	var (
		resp    *http.Response
		attempt int
	)

	err := c.retry.RunCtx(req.Context(), func(ctx context.Context) (err error) {
		if attempt++; attempt > 1 {
			if req.Body != http.NoBody && req.GetBody != nil {
				req.Body, err = req.GetBody()
				if err != nil {
					return fmt.Errorf("failed to get request body: %w", err)
				}
			}
		}

		tflog.Trace(ctx, "api call", map[string]interface{}{
			"attempt": attempt,
			"method":  req.Method,
			"url":     req.URL.String(),
		})

		resp, err = c.client.Do(req)
		if err != nil {
			return fmt.Errorf("request failed: %w", err)
		}

		if resp.StatusCode >= 300 {
			p, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, p)
		}

		if out == nil {
			return nil
		}

		if p, ok := out.(*[]byte); ok {
			*p, err = io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("reading body failed: %w", err)
			}
			return nil
		}

		return json.NewDecoder(resp.Body).Decode(out)
	})

	return resp, err
}

func (c *Client) BasicSign(req *http.Request, user string, secret []byte) error {
	mac := hmac.New(sha256.New, secret)

	if req.Body == nil || req.Body == http.NoBody {
		return errors.New("request body is empty")
	}

	if req.GetBody == nil {
		return errors.New("GetBody is nil, unable to rewind")
	}

	n, err := io.Copy(mac, req.Body)
	if n == 0 {
		return fmt.Errorf("error signing request body: body is empty")
	}
	if err != nil {
		return fmt.Errorf("error signing request body: %w", err)
	}

	digest := "v1." + hex.EncodeToString(mac.Sum(nil))

	req.SetBasicAuth(user, digest)

	req.Body, err = req.GetBody()
	if err != nil {
		return fmt.Errorf("error rewinding request body: %w", err)
	}

	return nil
}

func buildURL(format string, args ...any) string {
	u, err := url.Parse(fmt.Sprintf(format, args...))
	if err != nil {
		panic("unexpected error creating request: " + err.Error())
	}
	u.Path = strings.TrimRight(path.Join("/", u.Path), "/")
	return u.String()
}
