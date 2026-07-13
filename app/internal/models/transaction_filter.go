package models

// TxFilter holds optional query parameters for listing transactions.
type TxFilter struct {
	Search  string // case-insensitive substring match on description
	Type    string // exact match on transaction type
	From    string // "YYYY-MM-DD" lower bound on date (inclusive)
	To      string // "YYYY-MM-DD" upper bound on date (inclusive)
	Page    int    // 1-indexed; defaults to 1
	Limit   int    // rows per page; defaults to 50
	SortBy  string // column to sort by: date, description, type, amount, balance; defaults to date
	SortDir string // "asc" or "desc"; defaults to desc
}

// TxPage is the paginated response returned by the transactions endpoint.
type TxPage struct {
	Transactions []*Transaction `json:"transactions"`
	Total        int            `json:"total"`
	Page         int            `json:"page"`
	Limit        int            `json:"limit"`
	TotalPages   int            `json:"total_pages"`
}
