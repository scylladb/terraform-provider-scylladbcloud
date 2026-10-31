package scylla

import (
	"context"
	"fmt"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
)

func (c *Client) GetAccountNotificationEmails(ctx context.Context) ([]string, error) {
	var result model.NotificationEmails
	if err := c.get(ctx, fmt.Sprintf("/account/%d/notifications/email", c.AccountID), &result); err != nil {
		return nil, err
	}
	return result.Emails, nil
}

// SetAccountNotificationEmails replaces the account's notification emails;
// the API refuses an empty list.
func (c *Client) SetAccountNotificationEmails(ctx context.Context, emails []string) ([]string, error) {
	var result model.NotificationEmails
	if err := c.post(ctx, fmt.Sprintf("/account/%d/notifications/email", c.AccountID), model.NotificationEmails{Emails: emails}, &result); err != nil {
		return nil, err
	}
	return result.Emails, nil
}
