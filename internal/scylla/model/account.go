package model

// Account is the account the token belongs to, with the caller's role in it.
type Account struct {
	AccountID         int64  `json:"accountId"`
	UserID            int64  `json:"userId"`
	Name              string `json:"name"`
	UserAccountStatus string `json:"userAccountStatus"`
	Role              string `json:"role"`
	AccountStatus     string `json:"accountStatus"`
}
