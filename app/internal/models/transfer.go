package models

import (
	"time"

	"github.com/shopspring/decimal"
)

type MatchConfidence string

const (
	MatchByReference   MatchConfidence = "reference"
	MatchByShortNumber MatchConfidence = "short_number"
	MatchByDescription MatchConfidence = "description"
	MatchByAmountDate  MatchConfidence = "amount_date"
	MatchByFXRate      MatchConfidence = "fx_rate"
)

type TransferMatch struct {
	From       Transaction     `json:"from"`
	To         Transaction     `json:"to"`
	Confidence MatchConfidence `json:"confidence"`
}

// Transfer links two transactions representing the same money moving
// between two accounts (possibly across currencies in the future).
type Transfer struct {
	ID             string   `json:"id,omitempty"`
	FromTxID       string   `json:"from_tx_id"`
	ToTxID         string   `json:"to_tx_id"`
	ExchangeRate   *float64 `json:"exchange_rate,omitempty"`
	ExchangeSource string   `json:"exchange_source,omitempty"`
}

// TransferInput is the domain input for creating a new transfer between
// two accounts owned by the same user (e.g. loaning money to a borrower
// account, or moving money between two real accounts).
type TransferInput struct {
	FromAccountID string
	ToAccountID   string
	Amount        decimal.Decimal
	Currency      string
	Date          time.Time
	Description   string
}

// TransferResult is returned after successfully creating a transfer: the
// link row plus both transaction legs, so callers can update balances
// immediately without a extra round trip.
type TransferResult struct {
	Transfer        Transfer    `json:"transfer"`
	FromTransaction Transaction `json:"from_transaction"`
	ToTransaction   Transaction `json:"to_transaction"`
}
