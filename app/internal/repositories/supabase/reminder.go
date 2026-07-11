package supabase

import (
	"context"
	"net/url"
	"time"

	"ledger-api/app/internal/databases"
	"ledger-api/app/internal/models"
)

func NewReminderRepository(client *databases.SupabaseClient) *ReminderRepository {
	return &ReminderRepository{client: client}
}

func (r *ReminderRepository) List(ctx context.Context) ([]*models.Reminder, error) {
	thirtyDaysAgo := time.Now().UTC().AddDate(0, 0, -30).Format("2006-01-02")
	return databases.Get[[]*models.Reminder](ctx, r.client, "/rest/v1/payment_reminders", url.Values{
		"or":    []string{"(completed_at.is.null,completed_at.gte." + thirtyDaysAgo + ")"},
		"order": []string{"due_date.asc"},
	})
}

func (r *ReminderRepository) ListByAccountID(ctx context.Context, accountID string) ([]*models.Reminder, error) {
	thirtyDaysAgo := time.Now().UTC().AddDate(0, 0, -30).Format("2006-01-02")
	return databases.Get[[]*models.Reminder](ctx, r.client, "/rest/v1/payment_reminders", url.Values{
		"account_id": []string{"eq." + accountID},
		"or":         []string{"(completed_at.is.null,completed_at.gte." + thirtyDaysAgo + ")"},
		"order":      []string{"due_date.asc"},
	})
}

func (r *ReminderRepository) FindByID(ctx context.Context, id string) (*models.Reminder, error) {
	rows, err := databases.Get[[]*models.Reminder](ctx, r.client, "/rest/v1/payment_reminders", url.Values{
		"id":    []string{"eq." + id},
		"limit": []string{"1"},
	})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

func (r *ReminderRepository) Create(ctx context.Context, input models.ReminderInput) (*models.Reminder, error) {
	rows, err := databases.Post[[]*models.Reminder](ctx, r.client,
		"/rest/v1/payment_reminders",
		input,
		"return=representation")
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

func (r *ReminderRepository) Update(ctx context.Context, id string, fields map[string]any) (*models.Reminder, error) {
	rows, err := databases.Patch[[]*models.Reminder](ctx, r.client,
		"/rest/v1/payment_reminders?id=eq."+id,
		fields,
		"return=representation")
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

func (r *ReminderRepository) Delete(ctx context.Context, id string) error {
	return databases.Delete(ctx, r.client, "/rest/v1/payment_reminders?id=eq."+id)
}

func (r *ReminderRepository) MarkCompleted(ctx context.Context, id string) error {
	_, err := databases.Patch[struct{}](ctx, r.client,
		"/rest/v1/payment_reminders?id=eq."+id,
		map[string]any{"completed_at": time.Now().UTC().Format(time.RFC3339)},
		"return=minimal")
	return err
}
