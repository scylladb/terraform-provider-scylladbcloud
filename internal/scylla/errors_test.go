package scylla

import (
	"context"
	"net"
	"net/http"
	"os"
	"runtime"
	"syscall"
	"testing"
)

type fakeSyscallError struct {
	error
	temp bool
}

func (e fakeSyscallError) Error() string { return e.error.Error() }

func (e fakeSyscallError) Temporary() bool {
	return e.temp
}

func TestGetRetryInfo(t *testing.T) {
	t.Parallel()

	url := "https://someurl.com"

	type tcase struct {
		name     string
		err      error
		expected bool
	}

	var (
		ECONNABORTED = syscall.ECONNABORTED
		ECONNRESET   = syscall.ECONNRESET
	)

	if runtime.GOOS == "windows" {
		ECONNABORTED = syscall.Errno(10053) // WSAECONNABORTED
		ECONNRESET = syscall.Errno(10054)   // WSAECONNRESET
	}

	tcases := []tcase{
		{
			name:     "read body error",
			err:      makeAPIError("error reading body: "+"some reading error", errCodes, url, http.MethodGet, http.StatusOK, 0),
			expected: false,
		},
		{
			name:     "read body error [bad gateway]",
			err:      makeAPIError("error reading body: "+"some reading error", errCodes, url, http.MethodGet, http.StatusBadGateway, 0),
			expected: true,
		},
		{
			name:     "unmarshal error",
			err:      makeAPIError("failed to unmarshal data: "+"some unmarshal error", errCodes, url, http.MethodGet, http.StatusOK, 0),
			expected: false,
		},
		{
			name:     "unmarshal error [bad gateway]",
			err:      makeAPIError("failed to unmarshal data: "+"some unmarshal error", errCodes, url, http.MethodGet, http.StatusBadGateway, 0),
			expected: true,
		},
		{
			name:     "api error 000001",
			err:      makeAPIError("000001", errCodes, url, http.MethodGet, http.StatusOK, 0),
			expected: true,
		},
		{
			name:     "api error 021601",
			err:      makeAPIError("021601", errCodes, url, http.MethodGet, http.StatusOK, 0),
			expected: false,
		},
		{
			name: "net op os.SyscallError temp",
			err: &net.OpError{
				Op: "accept",
				Err: &os.SyscallError{
					Err: fakeSyscallError{
						error: syscall.ECONNABORTED,
						temp:  true,
					},
				},
			},
			expected: true,
		},
		{
			name: "net op os.SyscallError non-temp",
			err: &net.OpError{
				Op: "accept",
				Err: &os.SyscallError{
					Err: fakeSyscallError{
						error: syscall.ECONNABORTED,
						temp:  false,
					},
				},
			},
			expected: false,
		},
		{
			name: "net op temp",
			err: &net.OpError{
				Op: "accept",
				Err: fakeSyscallError{
					error: syscall.ECONNABORTED,
					temp:  true,
				},
			},
			expected: true,
		},
		{
			name: "net op non-temp",
			err: &net.OpError{
				Op: "accept",
				Err: fakeSyscallError{
					error: syscall.ECONNABORTED,
					temp:  false,
				},
			},
			expected: false,
		},
		{
			name: "net op accept error ECONNRESET",
			err: &net.OpError{
				Op:  "accept",
				Err: ECONNRESET,
			},
			expected: true,
		},
		{
			name: "net op accept error ECONNABORTED",
			err: &net.OpError{
				Op:  "accept",
				Err: ECONNABORTED,
			},
			expected: true,
		}}

	for id := range tcases {
		tc := tcases[id]
		t.Run(tc.name, func(t *testing.T) {
			isTemporary, _ := getRetryInfo(context.Background(), tc.err)
			if isTemporary != tc.expected {
				t.Errorf("expected %t, got %t", tc.expected, isTemporary)
			}
		})
	}
}
