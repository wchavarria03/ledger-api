package services

import (
	"context"
	"fmt"
	"time"

	"ledger-api/app/internal/auth"
	"ledger-api/app/internal/models"
)

func NewReminderService(reminders ReminderRepository) *ReminderService {
	return &ReminderService{reminders: reminders}
}

func (s *ReminderService) List(ctx context.Context) ([]models.ReminderWithStatus, error) {
	reminders, err := s.reminders.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list reminders: %w", err)
	}
	return toReminderStatuses(reminders), nil
}

func (s *ReminderService) ListByAccountID(ctx context.Context, accountID string) ([]models.ReminderWithStatus, error) {
	reminders, err := s.reminders.ListByAccountID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("list reminders by account: %w", err)
	}
	return toReminderStatuses(reminders), nil
}

func (s *ReminderService) Create(ctx context.Context, input models.ReminderInput) (*models.ReminderWithStatus, error) {
	userID := auth.UserIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("no authenticated user")
	}
	if input.Title == "" || input.DueDate == "" {
		return nil, fmt.Errorf("title and due_date are required")
	}
	if input.Amount != nil && (input.Currency == nil || *input.Currency == "") {
		return nil, fmt.Errorf("currency is required when amount is set")
	}
	if input.RecurrenceType != nil {
		if err := validateRecurrence(*input.RecurrenceType); err != nil {
			return nil, err
		}
	}
	input.UserID = userID
	r, err := s.reminders.Create(ctx, input)
	if err != nil {
		return nil, err
	}
	return toReminderWithStatus(r), nil
}

func (s *ReminderService) Update(ctx context.Context, id string, fields map[string]any) (*models.ReminderWithStatus, error) {
	if len(fields) == 0 {
		return nil, fmt.Errorf("no fields to update")
	}
	if rt, ok := fields["recurrence_type"].(string); ok {
		if err := validateRecurrence(rt); err != nil {
			return nil, err
		}
	}
	r, err := s.reminders.Update(ctx, id, fields)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, nil
	}
	return toReminderWithStatus(r), nil
}

func (s *ReminderService) Delete(ctx context.Context, id string) error {
	return s.reminders.Delete(ctx, id)
}

func (s *ReminderService) Complete(ctx context.Context, id string) (*models.ReminderWithStatus, error) {
	reminder, err := s.reminders.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find reminder: %w", err)
	}
	if reminder == nil {
		return nil, nil
	}

	if err := s.reminders.MarkCompleted(ctx, id); err != nil {
		return nil, fmt.Errorf("mark completed: %w", err)
	}

	if reminder.RecurrenceType != nil && *reminder.RecurrenceType != "" {
		nextDate := advanceReminderDate(reminder.DueDate, models.ReminderRecurrence(*reminder.RecurrenceType))
		nextInput := models.ReminderInput{
			UserID:         reminder.UserID,
			AccountID:      reminder.AccountID,
			Title:          reminder.Title,
			Amount:         reminder.Amount,
			Currency:       reminder.Currency,
			DueDate:        nextDate,
			RecurrenceType: reminder.RecurrenceType,
			Notes:          reminder.Notes,
		}
		next, err := s.reminders.Create(ctx, nextInput)
		if err != nil {
			return nil, fmt.Errorf("create next occurrence: %w", err)
		}
		if next != nil {
			if _, err := s.reminders.Update(ctx, id, map[string]any{"next_reminder_id": next.ID}); err != nil {
				return nil, fmt.Errorf("link next occurrence: %w", err)
			}
		}
	}

	updated, err := s.reminders.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, nil
	}
	return toReminderWithStatus(updated), nil
}

// reminderMatchWindow bounds how far a transaction's date may be from a
// reminder's due_date to still be considered a candidate match.
const reminderMatchWindow = 10 * 24 * time.Hour

// MatchCandidates finds resolved-but-unconfirmed reminders on accountID whose
// amount and currency exactly match one of txs and whose due_date falls
// within reminderMatchWindow of the transaction's date. Returned candidates
// are surfaced to the user for confirmation — nothing is linked here.
func (s *ReminderService) MatchCandidates(ctx context.Context, accountID string, txs []*models.Transaction) ([]models.ReminderMatch, error) {
	reminders, err := s.reminders.ListByAccountID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("list reminders: %w", err)
	}

	var matches []models.ReminderMatch
	for _, r := range reminders {
		if r.CompletedAt == nil || r.TransactionID != nil || r.Amount == nil {
			continue
		}
		dueDate, err := time.Parse("2006-01-02", r.DueDate)
		if err != nil {
			continue
		}
		for _, tx := range txs {
			if r.Currency != nil && *r.Currency != tx.Currency {
				continue
			}
			if !tx.Amount.Abs().Equal(r.Amount.Abs()) {
				continue
			}
			diff := tx.Date.Sub(dueDate)
			if diff < 0 {
				diff = -diff
			}
			if diff > reminderMatchWindow {
				continue
			}
			matches = append(matches, models.ReminderMatch{
				Reminder:    *toReminderWithStatus(r),
				Transaction: *tx,
			})
		}
	}
	return matches, nil
}

// Link confirms a resolved reminder by attaching the real transaction that
// paid it. If nextDueDate is set and the reminder has a next occurrence
// (created automatically at resolve time), that occurrence's due_date is
// updated too — for recurrences that shift based on the actual pay date
// rather than a fixed day.
func (s *ReminderService) Link(ctx context.Context, id, transactionID, nextDueDate string) (*models.ReminderWithStatus, error) {
	if transactionID == "" {
		return nil, fmt.Errorf("transaction_id is required")
	}
	reminder, err := s.reminders.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find reminder: %w", err)
	}
	if reminder == nil {
		return nil, nil
	}

	if _, err := s.reminders.Update(ctx, id, map[string]any{"transaction_id": transactionID}); err != nil {
		return nil, fmt.Errorf("link transaction: %w", err)
	}

	if nextDueDate != "" && reminder.NextReminderID != nil {
		if _, err := s.reminders.Update(ctx, *reminder.NextReminderID, map[string]any{"due_date": nextDueDate}); err != nil {
			return nil, fmt.Errorf("update next occurrence due date: %w", err)
		}
	}

	updated, err := s.reminders.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, nil
	}
	return toReminderWithStatus(updated), nil
}

func toReminderStatuses(reminders []*models.Reminder) []models.ReminderWithStatus {
	out := make([]models.ReminderWithStatus, len(reminders))
	for i, r := range reminders {
		out[i] = *toReminderWithStatus(r)
	}
	return out
}

func toReminderWithStatus(r *models.Reminder) *models.ReminderWithStatus {
	return &models.ReminderWithStatus{
		Reminder: *r,
		Status:   deriveReminderStatus(r),
	}
}

func deriveReminderStatus(r *models.Reminder) models.ReminderStatus {
	if r.CompletedAt != nil {
		if r.TransactionID == nil {
			return models.ReminderResolved
		}
		return models.ReminderCompleted
	}
	today := time.Now().UTC().Format("2006-01-02")
	switch {
	case r.DueDate < today:
		return models.ReminderOverdue
	case r.DueDate == today:
		return models.ReminderDueToday
	default:
		return models.ReminderUpcoming
	}
}

func advanceReminderDate(date string, recurrence models.ReminderRecurrence) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	switch recurrence {
	case models.ReminderWeekly:
		return t.AddDate(0, 0, 7).Format("2006-01-02")
	case models.ReminderBiweekly:
		return t.AddDate(0, 0, 14).Format("2006-01-02")
	case models.ReminderMonthly:
		return t.AddDate(0, 1, 0).Format("2006-01-02")
	case models.ReminderYearly:
		return t.AddDate(1, 0, 0).Format("2006-01-02")
	}
	return date
}

func validateRecurrence(rt string) error {
	switch models.ReminderRecurrence(rt) {
	case models.ReminderWeekly, models.ReminderBiweekly, models.ReminderMonthly, models.ReminderYearly:
		return nil
	}
	return fmt.Errorf("recurrence_type must be one of: weekly, biweekly, monthly, yearly")
}
