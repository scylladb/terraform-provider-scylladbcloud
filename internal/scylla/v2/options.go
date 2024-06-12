package scylla

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path"

	"github.com/eapache/go-resiliency/retrier"
	"golang.org/x/net/publicsuffix"
)

var globalCookieJar *cookiejar.Jar

func init() {
	var err error
	globalCookieJar, err = cookiejar.New(&cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	})
	if err != nil {
		panic("unexpected error: " + err.Error())
	}
}

func WithRetryPolicy(r *retrier.Retrier) func(*Client) {
	return func(c *Client) {
		c.retry = r
	}
}

func WithUserAgent(s string) func(*Client) {
	return func(c *Client) {
		c.reqmw = append(c.reqmw, func(r *http.Request) {
			r.Header.Set("User-Agent", s)
		})
	}
}

func WithGlobalCookieJar() func(*Client) {
	return func(c *Client) {
		c.client.Jar = globalCookieJar
	}
}

func WithCookieJar() func(*Client) {
	return func(c *Client) {
		jar, err := cookiejar.New(&cookiejar.Options{
			PublicSuffixList: publicsuffix.List,
		})
		if err != nil {
			panic("unexpected error: " + err.Error())
		}
		c.client.Jar = jar
	}
}

func WithBaseURL(s string) func(*Client) {
	return func(c *Client) {
		if s == "" {
			return
		}
		u, err := url.Parse(s)
		if err != nil {
			panic("unexpected error: " + err.Error())
		}
		c.reqmw = append(c.reqmw, func(r *http.Request) {
			if u.Scheme != "" {
				r.URL.Scheme = u.Scheme
			}
			if u.Host != "" {
				r.URL.Host = u.Host
			}
			if u.Path != "" {
				r.URL.Path = path.Join("/", u.Path, r.URL.Path)
			}
			if q := u.Query(); len(q) != 0 {
				orig := r.URL.Query()
				for k, v := range q {
					orig[k] = v
				}
				r.URL.RawQuery = orig.Encode()
			}
		})
	}
}
