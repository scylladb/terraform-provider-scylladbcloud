package scylla

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/eapache/go-resiliency/retrier"
	"github.com/hashicorp/terraform-plugin-log/tflogtest"
	"github.com/stretchr/testify/require"
)

// With TF_LOG=TRACE the client logs request and response bodies. Users share
// these logs, so the bodies must not hold the CQL password.
func TestTraceLogHasNoSecrets(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":{"broadcastType":"PUBLIC","credentials":{"username":"scylla","password":"cql-pass-1"},`+
			`"connectDataCenters":[{"dcName":"AWS_US_EAST_1","publicIPs":["192.0.2.1"]}]}}`)
	}))
	t.Cleanup(srv.Close)

	c, err := NewClient(srv.URL, "api-token-1", "test", "", false)
	require.NoError(t, err)
	c.AccountID = 1
	c.Retry = retrier.New(nil, DefaultClassifier)

	var logs bytes.Buffer
	ctx := tflogtest.RootLogger(context.Background(), &logs)

	conn, err := c.Connect(ctx, 3)
	require.NoError(t, err)
	require.Equal(t, "cql-pass-1", conn.Credentials.Password, "the caller still gets the password")

	require.NoError(t, c.post(ctx, "/account/1/cluster/3/user", map[string]any{"name": "u", "password": "req-pass-1"}, nil))

	out := logs.String()
	require.Contains(t, out, "scylla", "the log keeps the other fields")
	require.Contains(t, out, redacted)
	for _, secret := range []string{"cql-pass-1", "req-pass-1", "api-token-1"} {
		require.NotContains(t, out, secret)
	}
}

// A serverless connection bundle is a raw body with secrets that are not in
// JSON keys, so the log must not show any part of it.
func TestTraceLogHasNoRawBody(t *testing.T) {
	t.Parallel()
	const bundle = "authInfo:\n  username: scylla\n  password: bundle-pass-1\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, bundle)
	}))
	t.Cleanup(srv.Close)

	c, err := NewClient(srv.URL, "api-token-1", "test", "", false)
	require.NoError(t, err)
	c.AccountID = 1
	c.Retry = retrier.New(nil, DefaultClassifier)

	var logs bytes.Buffer
	ctx := tflogtest.RootLogger(context.Background(), &logs)

	got, err := c.Bundle(ctx, 3)
	require.NoError(t, err)
	require.Equal(t, bundle, string(got), "the caller still gets the bundle")

	out := logs.String()
	require.NotContains(t, out, "bundle-pass-1")
	require.NotContains(t, out, "authInfo")
	require.Contains(t, out, fmt.Sprintf(`"body":"[%d bytes, not JSON]"`, len(bundle)))
}

func TestLogBody(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ in, want string }{
		{in: `{"password":"x","name":"n"}`, want: `{"name":"n","password":"REDACTED"}`},
		{in: `{"data":[{"clientSecret":"x"},{"api_token":"y"}]}`, want: `{"data":[{"clientSecret":"REDACTED"},{"api_token":"REDACTED"}]}`},
		{in: `{"keyId":"k","n":12345678901234567890}`, want: `{"keyId":"k","n":12345678901234567890}`},
		{in: "-----BEGIN CERTIFICATE-----", want: "[27 bytes, not JSON]"},
		{in: `{"password":"cut-off`, want: "[20 bytes, not JSON]"},
		{in: `{"a":1} {"password":"x"}`, want: "[24 bytes, not JSON]"},
		{in: `[{"password":"x"},{"name":"n"}]`, want: `[{"password":"REDACTED"},{"name":"n"}]`},
		{in: `{"credentials":{"user":"u"},"secret":{"a":"b","c":["d"]}}`, want: `{"credentials":{"user":"u"},"secret":"REDACTED"}`},
		{in: "", want: ""},
	} {
		require.Equal(t, tt.want, logBody([]byte(tt.in)), tt.in)
	}
	cut := make([]byte, maxResponseBodyLength)
	require.Equal(t, fmt.Sprintf("[%d bytes, cut at the size limit]", maxResponseBodyLength), logBody(cut))
}

// An error answer from the API goes to the log too, so its secrets must be
// hidden as well.
func TestTraceLogOfAnErrorHasNoSecrets(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":"040001","data":{"password":"err-pass-1","name":"n"}}`)
	}))
	t.Cleanup(srv.Close)

	c, err := NewClient(srv.URL, "api-token-1", "test", "", false)
	require.NoError(t, err)
	c.AccountID = 1
	c.Retry = retrier.New(nil, DefaultClassifier)

	var logs bytes.Buffer
	ctx := tflogtest.RootLogger(context.Background(), &logs)

	_, err = c.Connect(ctx, 3)
	require.Error(t, err)

	out := logs.String()
	require.Contains(t, out, "api returned error")
	require.Contains(t, out, redacted)
	require.NotContains(t, out, "err-pass-1")
}
