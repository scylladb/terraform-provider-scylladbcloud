//go:build dev

package devauth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	appDir        = "terraform-provider-scylladbcloud"
	tokenLifetime = 24 * time.Hour
	minRemaining  = time.Hour
)

// Credentials of the local dev user.
type Credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// DefaultCredentialsFile is where scripts/local/stack.sh writes the dev user.
func DefaultCredentialsFile() string {
	return filepath.Join(xdgDir("XDG_CONFIG_HOME", ".config"), appDir, "dev-credentials.json")
}

// DefaultTokenCache is where minted tokens are cached between runs.
func DefaultTokenCache() string {
	return filepath.Join(xdgDir("XDG_CACHE_HOME", ".cache"), appDir, "dev-token.json")
}

func xdgDir(env, fallback string) string {
	if d := os.Getenv(env); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fallback
	}
	return filepath.Join(home, fallback)
}

// readPrivate reads a file that must not be accessible by group or others.
func readPrivate(path string) ([]byte, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if fi.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("%s has mode %o; it must be 0600", path, fi.Mode().Perm())
	}
	return os.ReadFile(path)
}

// LoadCredentials reads the credentials file written by stack.sh.
func LoadCredentials(path string) (Credentials, error) {
	var c Credentials
	b, err := readPrivate(path)
	if err != nil {
		return c, fmt.Errorf("reading dev credentials: %w", err)
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return c, fmt.Errorf("parsing dev credentials %s: %w", path, err)
	}
	if c.Email == "" || c.Password == "" {
		return c, fmt.Errorf("dev credentials %s: email and password are required", path)
	}
	return c, nil
}

type cachedToken struct {
	Token   string    `json:"token"`
	Expires time.Time `json:"expires"`
}

func cacheKey(sirenEndpoint, email string) string {
	return strings.TrimRight(sirenEndpoint, "/") + "|" + email
}

func loadCache(path string) map[string]cachedToken {
	m := map[string]cachedToken{}
	b, err := readPrivate(path)
	if err != nil {
		return m
	}
	_ = json.Unmarshal(b, &m)
	return m
}

func saveCache(path string, m map[string]cachedToken) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Login holds what is needed to mint a public API token from a local Siren.
type Login struct {
	SirenEndpoint string
	Credentials   Credentials
	CachePath     string
	HTTPClient    *http.Client
	Now           func() time.Time
}

// Token returns a cached token with at least an hour left, or mints a new one.
// Valid reports whether a cached token is still accepted; when it is not, the
// token is minted again.
func (l *Login) Token(ctx context.Context, valid func(string) bool) (string, error) {
	now := time.Now
	if l.Now != nil {
		now = l.Now
	}
	key := cacheKey(l.SirenEndpoint, l.Credentials.Email)
	cache := loadCache(l.CachePath)
	if t, ok := cache[key]; ok && t.Expires.Sub(now()) > minRemaining && (valid == nil || valid(t.Token)) {
		return t.Token, nil
	}

	session, err := l.post(ctx, "/user/login", "", map[string]string{
		"Email":    l.Credentials.Email,
		"Password": l.Credentials.Password,
	})
	if err != nil {
		return "", fmt.Errorf("dev login as %s: %w", l.Credentials.Email, err)
	}
	expires := now().Add(tokenLifetime).UTC().Truncate(time.Second)
	token, err := l.post(ctx, "/user/access-token", session, map[string]string{
		"Description":    "terraform-dev",
		"ExpirationDate": expires.Format(time.RFC3339),
	})
	if err != nil {
		return "", fmt.Errorf("dev access token: %w", err)
	}

	cache[key] = cachedToken{Token: token, Expires: expires}
	if err := saveCache(l.CachePath, cache); err != nil {
		return "", fmt.Errorf("caching dev token: %w", err)
	}
	return token, nil
}

type tokenResponse struct {
	Token string
	Data  *struct{ Token string } `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
}

func (l *Login) post(ctx context.Context, path, bearer string, body any) (string, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(l.SirenEndpoint, "/")+path, bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := l.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	var r tokenResponse
	if err := json.Unmarshal(raw, &r); err != nil {
		return "", fmt.Errorf("POST %s: status %d: unexpected body", path, resp.StatusCode)
	}
	if r.Error != nil && (r.Error.Code != "" || r.Error.Message != "") {
		return "", fmt.Errorf("POST %s: status %d: %s (%s)", path, resp.StatusCode, r.Error.Message, r.Error.Code)
	}
	token := r.Token
	if token == "" && r.Data != nil {
		token = r.Data.Token
	}
	if token == "" {
		return "", errors.New("POST " + path + ": no token in response")
	}
	return token, nil
}
