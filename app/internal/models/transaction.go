package models

import (
	"time"

	"github.com/shopspring/decimal"
)

type TransactionType string

const (
	TypeExpense  TransactionType = "expense"
	TypeIncome   TransactionType = "income"
	TypeTransfer TransactionType = "transfer"
)

type Transaction struct {
	ID          string          `json:"id,omitempty"`
	AccountID   string          `json:"account_id,omitempty"`
	Date        time.Time       `json:"date"`
	Reference   string          `json:"reference,omitempty"`
	Code        string          `json:"code,omitempty"`
	Type        TransactionType `json:"type"`
	Description string          `json:"description"`
	Amount      decimal.Decimal `json:"amount"`
	Balance     decimal.Decimal `json:"balance"`
	Currency    string          `json:"currency"`
	// ImportSeq preserves the row's position within its source statement
	// import, used as a tie-breaker when sorting same-date transactions
	// (the bare `date` column has no time component to order by).
	ImportSeq              *int        `json:"import_seq,omitempty"`
	TransferID             string      `json:"transfer_id,omitempty"`
	CounterpartAccountName string      `json:"counterpart_account_name,omitempty"`
	Categories             []*Category `json:"categories,omitempty"`
	// Note is a free-text annotation the user can attach when the imported
	// description isn't enough to identify what a purchase actually was.
	Note *string `json:"note,omitempty"`
}

type Statement struct {
	AccountNumber string
	ShortNumber   string
	SourceFile    string
	Transactions  []Transaction
}
