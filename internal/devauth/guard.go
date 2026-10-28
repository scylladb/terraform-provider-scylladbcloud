//go:build dev

// Package devauth implements the provider's dev mode: it logs in to a local
// Siren and refuses to talk to anything but the loopback interface. It is an
// accident guard, not a security boundary, and is compiled only with -tags dev.
package devauth

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// ProductionEndpoint is the provider's default endpoint, never allowed in dev mode.
const ProductionEndpoint = "https://api.cloud.scylladb.com"

var deniedHosts = map[string]bool{
	"host.docker.internal":    true,
	"gateway.docker.internal": true,
}

// Resolver looks up host addresses; it is a variable so tests can stub DNS.
var Resolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
} = net.DefaultResolver

// CheckEndpoint returns an error unless raw is an http(s) URL without
// credentials whose host resolves only to loopback addresses.
func CheckEndpoint(ctx context.Context, name, raw string) error {
	if raw == "" {
		return fmt.Errorf("%s must be set explicitly in dev mode", name)
	}
	if strings.TrimRight(raw, "/") == ProductionEndpoint {
		return fmt.Errorf("%s is the production endpoint %s; dev mode only talks to a local stack", name, ProductionEndpoint)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%s %q: scheme must be http or https", name, raw)
	}
	if u.User != nil {
		return fmt.Errorf("%s %q: credentials in the URL are not allowed", name, u.Redacted())
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("%s %q: missing host", name, raw)
	}
	if deniedHosts[strings.ToLower(strings.TrimSuffix(host, "."))] {
		return fmt.Errorf("%s %q: host %s is not loopback", name, raw, host)
	}
	if ip := net.ParseIP(host); ip != nil {
		if !ip.IsLoopback() {
			return fmt.Errorf("%s %q: %s is not a loopback address", name, raw, ip)
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	addrs, err := Resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("%s %q: %w", name, raw, err)
	}
	if len(addrs) == 0 {
		return fmt.Errorf("%s %q: %s resolves to no address", name, raw, host)
	}
	for _, a := range addrs {
		if !a.IP.IsLoopback() {
			return fmt.Errorf("%s %q: %s resolves to %s, which is not loopback", name, raw, host, a.IP)
		}
	}
	return nil
}

var errNotLoopback = errors.New("dev mode: refusing to connect to a non-loopback address")

func loopbackOnly(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("%w: %s", errNotLoopback, address)
	}
	return nil
}

// Transport returns an HTTP transport that ignores proxies and can only dial
// loopback addresses, whatever DNS says at request time.
func Transport() *http.Transport {
	d := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		Control:   loopbackOnly,
	}
	return &http.Transport{
		Proxy:                 nil,
		DialContext:           d.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}
