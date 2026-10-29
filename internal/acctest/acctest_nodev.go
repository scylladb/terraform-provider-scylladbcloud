//go:build !dev

package acctest

import (
	"context"
	"errors"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
)

const devBuild = false

func devLogin(context.Context, string, string) (string, []scylla.ClientOption, error) {
	return "", nil, errors.New("dev mode needs a build with -tags dev")
}
