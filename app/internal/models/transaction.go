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
	TransferID             string          `json:"transfer_id,omitempty"`
	CounterpartAccountName string          `json:"counterpart_account_name,omitempty"`
	Categories             []*Category     `json:"categories,omitempty"`
}

type Statement struct {
	AccountNumber string
	ShortNumber   string
	SourceFile    string
	Transactions  []Transaction
}
