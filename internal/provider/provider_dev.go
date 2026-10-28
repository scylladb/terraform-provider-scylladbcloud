//go:build dev

package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/devauth"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
)

const (
	defaultDevSirenEndpoint = "http://localhost:8080/api/v1"
	devModeHeader           = "X-Scylla-Dev-Mode"
)

func envDevMode() bool {
	v, _ := strconv.ParseBool(os.Getenv("SCYLLADB_CLOUD_DEV_MODE"))
	return v
}

func devSchema(s map[string]*schema.Schema) {
	// The token is optional in dev mode, so it is checked in devConfigure.
	s["token"].ValidateDiagFunc = nil

	s["dev_mode"] = &schema.Schema{
		Type:        schema.TypeBool,
		Optional:    true,
		Default:     envDevMode(),
		Description: "Dev builds only: log in to a local Siren and refuse non-loopback endpoints.",
	}
	s["dev_siren_endpoint"] = &schema.Schema{
		Type:        schema.TypeString,
		Optional:    true,
		Default:     nonempty(os.Getenv("SCYLLADB_CLOUD_DEV_SIREN_ENDPOINT"), defaultDevSirenEndpoint),
		Description: "Dev builds only: Siren REST API used to log in.",
	}
	s["dev_credentials_file"] = &schema.Schema{
		Type:        schema.TypeString,
		Optional:    true,
		Default:     devauth.DefaultCredentialsFile(),
		Description: "Dev builds only: JSON file (mode 0600) with the dev user's email and password.",
	}
}

func devConfigure(ctx context.Context, d *schema.ResourceData, endpoint, token string) (devSettings, diag.Diagnostics) {
	if !d.Get("dev_mode").(bool) {
		if token == "" {
			return devSettings{}, diag.Diagnostics{{
				Severity: diag.Error,
				Summary:  "token is required",
				Detail:   "A token must be provided to authenticate with the Scylla Cloud API.",
			}}
		}
		return devSettings{token: token}, nil
	}

	siren := d.Get("dev_siren_endpoint").(string)
	for name, u := range map[string]string{"endpoint": endpoint, "dev_siren_endpoint": siren} {
		if err := devauth.CheckEndpoint(ctx, name, u); err != nil {
			return devSettings{}, diag.Errorf("dev mode: %s", err)
		}
	}

	transport := devauth.Transport()
	httpClient := &http.Client{Timeout: 30 * time.Second, Transport: transport}

	if token == "" {
		creds, err := devauth.LoadCredentials(d.Get("dev_credentials_file").(string))
		if err != nil {
			return devSettings{}, diag.Errorf("dev mode: %s", err)
		}
		login := &devauth.Login{
			SirenEndpoint: siren,
			Credentials:   creds,
			CachePath:     devauth.DefaultTokenCache(),
			HTTPClient:    httpClient,
		}
		token, err = login.Token(ctx, func(t string) bool { return tokenAccepted(ctx, httpClient, endpoint, t) })
		if err != nil {
			return devSettings{}, diag.Errorf("dev mode: %s", err)
		}
	}

	tflog.Warn(ctx, "ScyllaDB Cloud provider is in dev mode", map[string]any{"endpoint": endpoint, "siren": siren})

	settings := devSettings{
		token:           token,
		userAgentSuffix: " dev",
		opts: []scylla.ClientOption{
			scylla.WithTransport(transport),
			scylla.WithHeader(devModeHeader, "1"),
		},
	}
	warning := diag.Diagnostic{
		Severity: diag.Warning,
		Summary:  "ScyllaDB Cloud provider is running in dev mode",
		Detail:   "Requests go to " + endpoint + " and the token comes from the local Siren at " + siren + ". Dev builds must never be used against ScyllaDB Cloud.",
	}
	return settings, diag.Diagnostics{warning}
}

// tokenAccepted reports whether the public API accepts a cached token.
func tokenAccepted(ctx context.Context, c *http.Client, endpoint, token string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(endpoint, "/")+"/account/default", nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set(devModeHeader, "1")
	resp, err := c.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var body struct {
		Data  json.RawMessage `json:"data"`
		Error json.RawMessage `json:"error"`
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil || json.Unmarshal(raw, &body) != nil {
		return false
	}
	return len(body.Data) > 0 && string(body.Data) != "null" && (len(body.Error) == 0 || string(body.Error) == "null")
}
