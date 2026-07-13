package models

type ImportPreview struct {
	AccountNumber    string        `json:"account_number"`
	Bank             string        `json:"bank"`
	Currency         string        `json:"currency"`
	TransactionCount int           `json:"transaction_count"`
	PeriodStart      string        `json:"period_start"`
	PeriodEnd        string        `json:"period_end"`
	Transactions     []Transaction `json:"transactions"`
	ExistingCount    int           `json:"existing_count"`
}

type ImportSummary struct {
	AccountName          string          `json:"account_name"`
	AccountNumber        string          `json:"account_number"`
	AccountIsNew         bool            `json:"account_is_new"`
	Currency             string          `json:"currency"`
	Bank                 string          `json:"bank"`
	ImportedCount        int             `json:"imported_count"`
	LinkedTransfersCount int             `json:"linked_transfers_count"`
	ReminderMatches      []ReminderMatch `json:"reminder_matches,omitempty"`
}

// TransactionOverride carries per-transaction corrections supplied by the user
// during the import review step. Index is 0-based and matches the Transactions
// slice returned by the dry-run.
type TransactionOverride struct {
	Index       int      `json:"index"`
	Date        string   `json:"date,omitempty"`         // YYYY-MM-DD
	Description string   `json:"description,omitempty"`
	Type        string   `json:"type,omitempty"`         // expense | income | transfer
	CategoryIDs []string `json:"category_ids,omitempty"`
}
