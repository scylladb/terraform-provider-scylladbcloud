package model

// BackupRetention is a cluster's backup retention after it is deleted.
type BackupRetention struct {
	RetentionDays  int64    `json:"retentionDays"`
	StartDate      string   `json:"startDate"`
	EncryptionKeys []string `json:"encryptionKeys"`
}
