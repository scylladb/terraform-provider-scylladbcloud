package scylla

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

func castToAPIError(err error) *APIError {
	var apiErr APIError
	if errors.As(err, &apiErr) {
		return &apiErr
	}
	return nil
}

func IsDeletedErr(err error) bool {
	if apiErr := castToAPIError(err); apiErr != nil {
		return apiErr.IsDeleted()
	}
	return false
}

// APIError represents an error that occurred while calling the API.
type APIError struct {
	URL        string
	Code       string
	Message    string
	Method     string
	StatusCode int
	RetryAfter time.Duration
}

func makeAPIError(text string, errCodes map[string]string, url, method string, statusCode int, retryAfter time.Duration) APIError {
	var err APIError
	if _, e := strconv.Atoi(text); e == nil {
		err.Code = text

		switch text := errCodes[text]; {
		case err.Message == "" && text == "":
			err.Message = "Request has failed. For more details consult the error code"
		case err.Message == "":
			err.Message = text
		case text != "":
			err.Message = err.Message + " (" + text + ")"
		}
	} else {
		err.Message = text
	}
	if err.URL == "" {
		err.URL = url
	}
	if err.StatusCode == 0 {
		err.StatusCode = statusCode
	}
	err.Method = method
	err.RetryAfter = retryAfter
	return err
}

func (err APIError) Temporary() bool {
	switch err.StatusCode {
	case http.StatusBadGateway, http.StatusGatewayTimeout, http.StatusTooManyRequests, http.StatusServiceUnavailable:
		return true
	}
	return err.Code == "000001"
}

func (err APIError) IsDeleted() bool {
	return err.Code == "040001"
}

func (err APIError) Error() string {
	return fmt.Sprintf("Error %q: %s (http status %d, method %s url %q)", err.Code, err.Message, err.StatusCode, err.Method, err.URL)
}
