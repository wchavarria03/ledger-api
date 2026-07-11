package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// Budget is the stored shape — one spending limit per (user, category, currency).
type Budget struct {
	ID            string          `json:"id"`
	UserID        string          `json:"user_id,omitempty"`
	CategoryID    string          `json:"category_id"`
	CategoryName  string          `json:"category_name,omitempty"`
	CategoryColor string          `json:"category_color,omitempty"`
	Currency      string          `json:"currency"`
	Amount        decimal.Decimal `json:"amount"`
	CreatedAt     time.Time       `json:"created_at,omitempty"`
}

// BudgetStatus is Budget enriched with computed spending data for a specific month.
type BudgetStatus struct {
	Budget
	Spent        decimal.Decimal `json:"spent"`
	Remaining    decimal.Decimal `json:"remaining"`
	Percent      float64         `json:"percent"`
	Status       string          `json:"status"` // on_track | warning | exceeded
	Acknowledged bool            `json:"acknowledged"`
}

// BudgetInput is the write shape when creating a budget.
type BudgetInput struct {
	UserID     string          `json:"user_id,omitempty"`
	CategoryID string          `json:"category_id"`
	Currency   string          `json:"currency"`
	Amount     decimal.Decimal `json:"amount"`
}

// BudgetAcknowledgment records what the user decided to do with an underspend.
type BudgetAcknowledgment struct {
	ID         string    `json:"id"`
	BudgetID   string    `json:"budget_id"`
	Month      string    `json:"month"`
	Action     string    `json:"action"` // kept | moved
	TransferID *string   `json:"transfer_id,omitempty"`
	CreatedAt  time.Time `json:"created_at,omitempty"`
}
