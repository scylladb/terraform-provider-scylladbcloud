//go:build dev

package acctest

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/devauth"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
)

const devBuild = true

// devLogin mirrors the provider's dev mode: loopback-only endpoints and, when
// no token is set, a token minted from the local Siren.
func devLogin(ctx context.Context, endpoint, token string) (string, []scylla.ClientOption, error) {
	siren := os.Getenv("SCYLLADB_CLOUD_DEV_SIREN_ENDPOINT")
	if siren == "" {
		siren = "http://localhost:8080/api/v1"
	}
	for name, u := range map[string]string{"endpoint": endpoint, "dev siren endpoint": siren} {
		if err := devauth.CheckEndpoint(ctx, name, u); err != nil {
			return "", nil, fmt.Errorf("dev mode: %w", err)
		}
	}
	transport := devauth.Transport()
	if token == "" {
		creds, err := devauth.LoadCredentials(devauth.DefaultCredentialsFile())
		if err != nil {
			return "", nil, fmt.Errorf("dev mode: %w", err)
		}
		login := &devauth.Login{
			SirenEndpoint: siren,
			Credentials:   creds,
			CachePath:     devauth.DefaultTokenCache(),
			HTTPClient:    &http.Client{Timeout: 30 * time.Second, Transport: transport},
		}
		if token, err = login.Token(ctx, nil); err != nil {
			return "", nil, fmt.Errorf("dev mode: %w", err)
		}
	}
	return token, []scylla.ClientOption{scylla.WithTransport(transport), scylla.WithHeader("X-Scylla-Dev-Mode", "1")}, nil
}
