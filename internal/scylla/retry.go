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

	if e := (temp)(nil); errors.As(err, &e) && e.Temporary() {
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
