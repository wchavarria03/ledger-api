package models

import (
	"time"

	"github.com/shopspring/decimal"
)

type RecurrenceType string

const (
	RecurrenceMonthly  RecurrenceType = "monthly"
	RecurrenceBiweekly RecurrenceType = "biweekly"
)

// Envelope is the stored shape — a named savings bucket inside a real account.
type Envelope struct {
	ID                   string           `json:"id"`
	UserID               string           `json:"user_id,omitempty"`
	AccountID            string           `json:"account_id"`
	Name                 string           `json:"name"`
	TargetAmount         *decimal.Decimal `json:"target_amount,omitempty"`
	Currency             string           `json:"currency"`
	RecurringAmount      *decimal.Decimal `json:"recurring_amount,omitempty"`
	RecurrenceType       string           `json:"recurrence_type,omitempty"`
	NextContributionDate *string          `json:"next_contribution_date,omitempty"` // YYYY-MM-DD
	CreatedAt            time.Time        `json:"created_at,omitempty"`
}

// EnvelopeStatus is Envelope enriched with computed balance and due state.
type EnvelopeStatus struct {
	Envelope
	Balance decimal.Decimal `json:"balance"`
	Percent *float64        `json:"percent,omitempty"` // nil when no target set
	IsDue   bool            `json:"is_due"`
}

// EnvelopeInput is the write shape for create and update.
type EnvelopeInput struct {
	UserID               string           `json:"user_id,omitempty"`
	AccountID            string           `json:"account_id,omitempty"`
	Name                 string           `json:"name,omitempty"`
	TargetAmount         *decimal.Decimal `json:"target_amount,omitempty"`
	Currency             string           `json:"currency,omitempty"`
	RecurringAmount      *decimal.Decimal `json:"recurring_amount,omitempty"`
	RecurrenceType       string           `json:"recurrence_type,omitempty"`
	NextContributionDate *string          `json:"next_contribution_date,omitempty"`
}

// EnvelopeContribution is a single deposit or withdrawal into an envelope.
type EnvelopeContribution struct {
	ID         string          `json:"id"`
	EnvelopeID string          `json:"envelope_id"`
	Amount     decimal.Decimal `json:"amount"`
	Note       string          `json:"note,omitempty"`
	Date       string          `json:"date"`
	CreatedAt  time.Time       `json:"created_at,omitempty"`
}

// ContributionInput is the write shape for a new contribution.
type ContributionInput struct {
	Amount         decimal.Decimal `json:"amount"`
	Note           string          `json:"note,omitempty"`
	Date           string          `json:"date"`
	ApplyRecurring bool            `json:"apply_recurring"`
}
