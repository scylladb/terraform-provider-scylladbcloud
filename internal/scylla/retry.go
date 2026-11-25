package scylla

import (
	"errors"
	"net"
	"net/http"
	"slices"
	"strings"

	"github.com/eapache/go-resiliency/retrier"
)

var DefaultClassifier retrier.Classifier = &RetryStrategy{
	RetryStatus: []int{
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
		// http.StatusInternalServerError,
		http.StatusTooManyRequests,
		http.StatusRequestTimeout,
		http.StatusTooEarly,
		http.StatusLocked,
	},
	RetryCode: []string{
		"000001",
	},
	RetryMessage: []string{
		"connection reset",
		"use of closed network connection",
		"broken pipe",
		"transport connection broken",
		"connection refused",
	},
	FailMessage: []string{
		"certificate is not trusted",
		"unsupported protocol scheme",
		"net/http: request canceled",
		"net/http: request canceled while waiting for connection",
	},
}

// OnceClassifier is for requests that the API must not run twice, such as a
// create. It fails when the API may have run the request. Else it uses
// DefaultClassifier.
var OnceClassifier retrier.Classifier = onceClassifier{next: DefaultClassifier}

type onceClassifier struct {
	next retrier.Classifier
}

func (o onceClassifier) Classify(err error) retrier.Action {
	if MayHaveRun(err) {
		return retrier.Fail
	}
	return o.next.Classify(err)
}

// acceptedError is an error that comes after the API accepted a request, for
// example when the next call to read the request fails. The API runs the
// request in all cases.
type acceptedError struct{ err error }

func (e *acceptedError) Error() string { return e.err.Error() }
func (e *acceptedError) Unwrap() error { return e.err }

// MayHaveRun reports whether the API may have run a request that failed with
// err. Only a request that was not sent, or a 4xx answer other than 408, shows
// that the API did not run it. An error after the API accepted the request is
// always true, whatever its code. It returns false for a nil err.
func MayHaveRun(err error) bool {
	if err == nil {
		return false
	}
	if e := (*acceptedError)(nil); errors.As(err, &e) {
		return true
	}
	if e := (*APIError)(nil); errors.As(err, &e) {
		return e.StatusCode < 400 || e.StatusCode >= 500 || e.StatusCode == http.StatusRequestTimeout
	}
	if e := (*net.OpError)(nil); errors.As(err, &e) && e.Op == "dial" {
		return false
	}
	return true
}

type RetryStrategy struct {
	RetryStatus  []int
	RetryCode    []string
	RetryMessage []string
	FailMessage  []string
}

func (rs *RetryStrategy) Classify(err error) retrier.Action {
	if err == nil {
		return retrier.Succeed
	}

	if e := (*APIError)(nil); errors.As(err, &e) {
		if slices.Contains(rs.RetryCode, e.Code) {
			return retrier.Retry
		}

		if slices.Contains(rs.RetryStatus, e.StatusCode) {
			return retrier.Retry
		}
	}

	type temp interface {
		Temporary() bool
	}

	if e := temp(nil); errors.As(err, &e) && e.Temporary() {
		return retrier.Retry
	}

	if e := (*net.OpError)(nil); errors.As(err, &e) && e.Op == "dial" {
		return retrier.Retry
	}

	for _, msg := range rs.FailMessage {
		if strings.Contains(err.Error(), msg) {
			return retrier.Fail
		}
	}

	for _, msg := range rs.RetryMessage {
		if strings.Contains(err.Error(), msg) {
			return retrier.Retry
		}
	}

	return retrier.Fail
}
