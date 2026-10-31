package backupretention

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla/model"
)

// errCodeNotFound is returned when a cluster has no backup retention.
const errCodeNotFound = "045002"

func ResourceBackupRetention() *schema.Resource {
	return &schema.Resource{
		Description: "Manages how long the backups of a deleted cluster are kept. " +
			"A cluster has a backup retention only after it is deleted with `backup_retention_days` above 0; " +
			"destroying this resource leaves the retention as it is.",

		CreateContext: resourceBackupRetentionCreate,
		ReadContext:   resourceBackupRetentionRead,
		UpdateContext: resourceBackupRetentionUpdate,
		DeleteContext: resourceBackupRetentionDelete,

		Importer: &schema.ResourceImporter{
			StateContext: resourceBackupRetentionImport,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(5 * time.Minute),
			Update: schema.DefaultTimeout(5 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"cluster_id": {
				Description: "ID of the deleted cluster whose backups are kept.",
				Required:    true,
				ForceNew:    true,
				Type:        schema.TypeInt,
			},
			"retention_days": {
				Description: "Days to keep the backups, counted from `start_date`, between 0 and 60. " +
					"0 cancels the retention and the backups are deleted at the next check.",
				Required:         true,
				Type:             schema.TypeInt,
				ValidateDiagFunc: validation.ToDiagFunc(validation.IntBetween(0, 60)),
			},
			"start_date": {
				Description: "When the retention started (RFC 3339).",
				Computed:    true,
				Type:        schema.TypeString,
			},
			"encryption_keys": {
				Description: "IDs of the encryption keys kept for the backups.",
				Computed:    true,
				Type:        schema.TypeList,
				Elem:        &schema.Schema{Type: schema.TypeString},
			},
		},
	}
}

func isNotFound(err error) bool {
	var apiErr *scylla.APIError
	return errors.As(err, &apiErr) && apiErr.Code == errCodeNotFound
}

func setRetention(d *schema.ResourceData, r *model.BackupRetention) {
	_ = d.Set("retention_days", r.RetentionDays)
	_ = d.Set("start_date", r.StartDate)
	_ = d.Set("encryption_keys", r.EncryptionKeys)
}

func resourceBackupRetentionCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c := meta.(*scylla.Client)
	clusterID := int64(d.Get("cluster_id").(int))

	current, err := c.GetBackupRetention(ctx, clusterID)
	if isNotFound(err) {
		return diag.Errorf("cluster %d has no backup retention: it exists only after the cluster is deleted "+
			"with backup_retention_days above 0, and until it expires", clusterID)
	}
	if err != nil {
		return diag.Errorf("failed to read backup retention of cluster %d: %s", clusterID, err)
	}

	d.SetId(strconv.FormatInt(clusterID, 10))
	if want := int64(d.Get("retention_days").(int)); want != current.RetentionDays {
		if current, err = c.UpdateBackupRetention(ctx, clusterID, want); err != nil {
			d.SetId("")
			return diag.Errorf("failed to set backup retention of cluster %d to %d days: %s", clusterID, want, err)
		}
	}
	setRetention(d, current)
	return nil
}

func resourceBackupRetentionRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c := meta.(*scylla.Client)
	clusterID, err := strconv.ParseInt(d.Id(), 10, 64)
	if err != nil {
		return diag.Errorf("invalid backup retention ID %q: %s", d.Id(), err)
	}

	r, err := c.GetBackupRetention(ctx, clusterID)
	if isNotFound(err) {
		d.SetId("")
		return nil
	}
	if err != nil {
		return diag.Errorf("failed to read backup retention of cluster %d: %s", clusterID, err)
	}

	_ = d.Set("cluster_id", clusterID)
	setRetention(d, r)
	return nil
}

func resourceBackupRetentionUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c := meta.(*scylla.Client)
	clusterID := int64(d.Get("cluster_id").(int))
	want := int64(d.Get("retention_days").(int))

	r, err := c.UpdateBackupRetention(ctx, clusterID, want)
	if err != nil {
		return diag.Errorf("failed to set backup retention of cluster %d to %d days: %s", clusterID, want, err)
	}
	setRetention(d, r)
	return nil
}

// resourceBackupRetentionDelete only forgets the retention: removing it would
// either delete the backups or leave them, which a destroy should not decide.
func resourceBackupRetentionDelete(context.Context, *schema.ResourceData, interface{}) diag.Diagnostics {
	return nil
}

func resourceBackupRetentionImport(_ context.Context, d *schema.ResourceData, _ interface{}) ([]*schema.ResourceData, error) {
	id, err := strconv.ParseInt(d.Id(), 10, 64)
	if err != nil {
		return nil, errors.New("import ID must be the cluster ID")
	}
	_ = d.Set("cluster_id", id)
	return []*schema.ResourceData{d}, nil
}
