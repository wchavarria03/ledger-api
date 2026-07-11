package models

import (
	"time"

	"github.com/shopspring/decimal"
)

type ReminderStatus string

const (
	ReminderOverdue   ReminderStatus = "overdue"
	ReminderDueToday  ReminderStatus = "due_today"
	ReminderUpcoming  ReminderStatus = "upcoming"
	ReminderCompleted ReminderStatus = "completed"
)

type ReminderRecurrence string

const (
	ReminderWeekly   ReminderRecurrence = "weekly"
	ReminderBiweekly ReminderRecurrence = "biweekly"
	ReminderMonthly  ReminderRecurrence = "monthly"
	ReminderYearly   ReminderRecurrence = "yearly"
)

// Reminder is the stored shape from payment_reminders.
type Reminder struct {
	ID             string           `json:"id"`
	UserID         string           `json:"user_id,omitempty"`
	AccountID      *string          `json:"account_id,omitempty"`
	Title          string           `json:"title"`
	Amount         *decimal.Decimal `json:"amount,omitempty"`
	Currency       *string          `json:"currency,omitempty"`
	DueDate        string           `json:"due_date"` // YYYY-MM-DD
	RecurrenceType *string          `json:"recurrence_type,omitempty"`
	CompletedAt    *time.Time       `json:"completed_at,omitempty"`
	Notes          *string          `json:"notes,omitempty"`
	CreatedAt      time.Time        `json:"created_at,omitempty"`
}

// ReminderWithStatus is Reminder enriched with a derived status field.
type ReminderWithStatus struct {
	Reminder
	Status ReminderStatus `json:"status"`
}

// ReminderInput is the write shape for create and update.
type ReminderInput struct {
	UserID         string           `json:"user_id,omitempty"`
	AccountID      *string          `json:"account_id,omitempty"`
	Title          string           `json:"title,omitempty"`
	Amount         *decimal.Decimal `json:"amount,omitempty"`
	Currency       *string          `json:"currency,omitempty"`
	DueDate        string           `json:"due_date,omitempty"`
	RecurrenceType *string          `json:"recurrence_type,omitempty"`
	Notes          *string          `json:"notes,omitempty"`
}
