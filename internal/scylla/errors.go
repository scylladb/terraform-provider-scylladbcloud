package scylla

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
)

func IsClusterDeletedErr(err error) bool {
	if e := new(APIError); errors.As(err, &e) && e.Message == "CLUSTER_DELETED" {
		return true
	}
	return false
}

func IsDeletedErr(err error) bool {
	if e := new(APIError); errors.As(err, &e) && e.Code == "040001" {
		return true
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
}

func makeError(text string, errCodes map[string]string, r *http.Response) *APIError {
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
		err.URL = r.Request.URL.String()
	}
	if err.StatusCode == 0 {
		err.StatusCode = r.StatusCode
	}
	err.Method = r.Request.Method
	return &err
}

func (err *APIError) Error() string {
	return fmt.Sprintf("Error %q: %s (http status %d, method %s url %q)", err.Code, err.Message, err.StatusCode, err.Method, err.URL)
}
