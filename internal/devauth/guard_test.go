//go:build dev

package devauth

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeResolver map[string][]string

func (f fakeResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	ips, ok := f[host]
	if !ok {
		return nil, errors.New("no such host")
	}
	out := make([]net.IPAddr, 0, len(ips))
	for _, ip := range ips {
		out = append(out, net.IPAddr{IP: net.ParseIP(ip)})
	}
	return out, nil
}

func TestCheckEndpoint(t *testing.T) {
	orig := Resolver
	t.Cleanup(func() { Resolver = orig })
	Resolver = fakeResolver{
		"localhost":       {"127.0.0.1", "::1"},
		"mixed.test":      {"127.0.0.1", "203.0.113.5"},
		"remote.test":     {"203.0.113.5"},
		"empty.test":      {},
		"api.example.com": {"198.51.100.1"},
	}

	for _, tc := range []struct {
		url string
		ok  bool
	}{
		{"http://localhost:3011", true},
		{"http://127.0.0.1:8080/api/v1", true},
		{"http://[::1]:3011", true},
		{"https://localhost", true},
		{"", false},
		{"https://api.cloud.scylladb.com", false},
		{"https://api.cloud.scylladb.com/", false},
		{"http://user:pw@localhost:3011", false},
		{"ftp://localhost", false},
		{"http://", false},
		{"http://203.0.113.5:3011", false},
		{"http://mixed.test", false},
		{"http://remote.test", false},
		{"http://empty.test", false},
		{"http://unknown.test", false},
		{"http://host.docker.internal:3011", false},
		{"http://HOST.DOCKER.INTERNAL.:3011", false},
		{"https://api.example.com", false},
	} {
		err := CheckEndpoint(context.Background(), "endpoint", tc.url)
		if tc.ok {
			require.NoError(t, err, tc.url)
		} else {
			require.Error(t, err, tc.url)
		}
	}
}

func TestLoopbackOnly(t *testing.T) {
	require.NoError(t, loopbackOnly("tcp", "127.0.0.1:80", nil))
	require.NoError(t, loopbackOnly("tcp", "[::1]:80", nil))
	require.ErrorIs(t, loopbackOnly("tcp", "192.0.2.1:80", nil), errNotLoopback)
	require.Error(t, loopbackOnly("tcp", "garbage", nil))
}

func TestTransportDialsLoopbackOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()

	c := &http.Client{Transport: Transport()}
	resp, err := c.Get(srv.URL)
	require.NoError(t, err)
	resp.Body.Close()

	_, err = c.Get("http://192.0.2.1:9/")
	require.ErrorIs(t, err, errNotLoopback)
}
