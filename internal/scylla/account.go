package scylla

import (
	"context"
	"fmt"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
)

// GetDefaultAccount returns the account the token belongs to.
func (c *Client) GetDefaultAccount(ctx context.Context) (*model.Account, error) {
	var result model.Account
	if err := c.get(ctx, "/account/default", &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetAccountCertificate(ctx context.Context) (*model.ClusterCertificate, error) {
	var result model.ClusterCertificate
	if err := c.get(ctx, fmt.Sprintf("/account/%d/certificate", c.AccountID), &result); err != nil {
		return nil, err
	}
	return &result, nil
}
