//go:build dev

package devauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeSiren struct {
	logins, mints atomic.Int32
	password      string
}

func (f *fakeSiren) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body map[string]string
	_ = json.NewDecoder(r.Body).Decode(&body)
	switch r.URL.Path {
	case "/api/v1/user/login":
		if body["Password"] != f.password {
			_ = json.NewEncoder(w).Encode(map[string]any{"Error": map[string]string{"code": "020001", "message": "bad credentials"}})
			return
		}
		f.logins.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]string{"Token": "session"})
	case "/api/v1/user/access-token":
		if r.Header.Get("Authorization") != "Bearer session" || body["ExpirationDate"] == "" {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{"Error": map[string]string{"code": "020006", "message": "unauthorized"}})
			return
		}
		n := f.mints.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]string{"Token": "access-" + string(rune('0'+n))})
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func newLogin(t *testing.T, f *fakeSiren, now time.Time) *Login {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return &Login{
		SirenEndpoint: srv.URL + "/api/v1",
		Credentials:   Credentials{Email: "tf-dev@local.test", Password: "pw"},
		CachePath:     filepath.Join(t.TempDir(), "cache", "dev-token.json"),
		HTTPClient:    &http.Client{Transport: Transport()},
		Now:           func() time.Time { return now },
	}
}

func TestLoginMintsAndCaches(t *testing.T) {
	f := &fakeSiren{password: "pw"}
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	l := newLogin(t, f, now)

	tok, err := l.Token(context.Background(), nil)
	require.NoError(t, err)
	require.Equal(t, "access-1", tok)

	fi, err := os.Stat(l.CachePath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), fi.Mode().Perm())

	tok, err = l.Token(context.Background(), func(string) bool { return true })
	require.NoError(t, err)
	require.Equal(t, "access-1", tok)
	require.Equal(t, int32(1), f.mints.Load())
}

func TestLoginRemintsRejectedOrExpiring(t *testing.T) {
	f := &fakeSiren{password: "pw"}
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	l := newLogin(t, f, now)
	_, err := l.Token(context.Background(), nil)
	require.NoError(t, err)

	tok, err := l.Token(context.Background(), func(string) bool { return false })
	require.NoError(t, err)
	require.Equal(t, "access-2", tok)

	l.Now = func() time.Time { return now.Add(23*time.Hour + 30*time.Minute) }
	tok, err = l.Token(context.Background(), func(string) bool { return true })
	require.NoError(t, err)
	require.Equal(t, "access-3", tok)
}

func TestLoginError(t *testing.T) {
	f := &fakeSiren{password: "other"}
	l := newLogin(t, f, time.Now())
	_, err := l.Token(context.Background(), nil)
	require.ErrorContains(t, err, "bad credentials")
}

func TestLoadCredentials(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "creds.json")
	require.NoError(t, os.WriteFile(p, []byte(`{"email":"a@local.test","password":"x"}`), 0o600))
	c, err := LoadCredentials(p)
	require.NoError(t, err)
	require.Equal(t, Credentials{Email: "a@local.test", Password: "x"}, c)

	require.NoError(t, os.Chmod(p, 0o644))
	_, err = LoadCredentials(p)
	require.ErrorContains(t, err, "must be 0600")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "empty.json"), []byte(`{"email":"a@local.test"}`), 0o600))
	_, err = LoadCredentials(filepath.Join(dir, "empty.json"))
	require.ErrorContains(t, err, "required")
}

func TestDefaultPathsHonourXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/cfg")
	t.Setenv("XDG_CACHE_HOME", "/cache")
	require.Equal(t, "/cfg/terraform-provider-scylladbcloud/dev-credentials.json", DefaultCredentialsFile())
	require.Equal(t, "/cache/terraform-provider-scylladbcloud/dev-token.json", DefaultTokenCache())
}
