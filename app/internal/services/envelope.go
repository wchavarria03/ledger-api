package services

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"ledger-api/app/internal/auth"
	"ledger-api/app/internal/models"
)

func NewEnvelopeService(envelopes EnvelopeRepository) *EnvelopeService {
	return &EnvelopeService{envelopes: envelopes}
}

func (s *EnvelopeService) List(ctx context.Context) ([]models.EnvelopeStatus, error) {
	envelopes, err := s.envelopes.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list envelopes: %w", err)
	}
	return s.enrich(ctx, envelopes)
}

func (s *EnvelopeService) ListByAccountID(ctx context.Context, accountID string) ([]models.EnvelopeStatus, error) {
	envelopes, err := s.envelopes.ListByAccountID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("list envelopes by account: %w", err)
	}
	return s.enrich(ctx, envelopes)
}

func (s *EnvelopeService) Create(ctx context.Context, input models.EnvelopeInput) (*models.EnvelopeStatus, error) {
	userID := auth.UserIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("no authenticated user")
	}
	if input.AccountID == "" || input.Name == "" || input.Currency == "" {
		return nil, fmt.Errorf("account_id, name, and currency are required")
	}
	if input.RecurrenceType != "" &&
		input.RecurrenceType != string(models.RecurrenceMonthly) &&
		input.RecurrenceType != string(models.RecurrenceBiweekly) {
		return nil, fmt.Errorf("recurrence_type must be 'monthly' or 'biweekly'")
	}
	input.UserID = userID
	env, err := s.envelopes.Create(ctx, input)
	if err != nil {
		return nil, err
	}
	return envelopeToStatus(env, decimal.Zero), nil
}

func (s *EnvelopeService) Update(ctx context.Context, id string, fields map[string]any) (*models.EnvelopeStatus, error) {
	env, err := s.envelopes.Update(ctx, id, fields)
	if err != nil {
		return nil, err
	}
	if env == nil {
		return nil, nil
	}
	balances, err := s.envelopes.GetBalances(ctx, []string{id})
	if err != nil {
		return nil, err
	}
	return envelopeToStatus(env, balances[id]), nil
}

func (s *EnvelopeService) Delete(ctx context.Context, id string) error {
	return s.envelopes.Delete(ctx, id)
}

func (s *EnvelopeService) Contribute(ctx context.Context, id string, input models.ContributionInput) (*models.EnvelopeStatus, error) {
	if input.Amount.IsZero() {
		return nil, fmt.Errorf("amount cannot be zero")
	}
	if input.Date == "" {
		input.Date = time.Now().UTC().Format("2006-01-02")
	}

	if _, err := s.envelopes.Contribute(ctx, id, input); err != nil {
		return nil, err
	}

	if input.ApplyRecurring {
		env, err := s.envelopes.FindByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if env != nil && env.RecurrenceType != "" && env.NextContributionDate != nil {
			next := advanceDate(*env.NextContributionDate, models.RecurrenceType(env.RecurrenceType))
			if err := s.envelopes.SetNextContributionDate(ctx, id, next); err != nil {
				return nil, err
			}
		}
	}

	env, err := s.envelopes.FindByID(ctx, id)
	if err != nil || env == nil {
		return nil, err
	}
	balances, err := s.envelopes.GetBalances(ctx, []string{id})
	if err != nil {
		return nil, err
	}
	return envelopeToStatus(env, balances[id]), nil
}

// enrich fetches balances for a slice of envelopes and builds EnvelopeStatus values.
func (s *EnvelopeService) enrich(ctx context.Context, envelopes []*models.Envelope) ([]models.EnvelopeStatus, error) {
	if len(envelopes) == 0 {
		return []models.EnvelopeStatus{}, nil
	}
	ids := make([]string, len(envelopes))
	for i, e := range envelopes {
		ids[i] = e.ID
	}
	balances, err := s.envelopes.GetBalances(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("get balances: %w", err)
	}
	statuses := make([]models.EnvelopeStatus, len(envelopes))
	for i, e := range envelopes {
		statuses[i] = *envelopeToStatus(e, balances[e.ID])
	}
	return statuses, nil
}

func envelopeToStatus(e *models.Envelope, balance decimal.Decimal) *models.EnvelopeStatus {
	today := time.Now().UTC().Format("2006-01-02")
	status := &models.EnvelopeStatus{
		Envelope: *e,
		Balance:  balance,
		IsDue:    e.NextContributionDate != nil && *e.NextContributionDate <= today,
	}
	if e.TargetAmount != nil && e.TargetAmount.IsPositive() {
		pct, _ := balance.Div(*e.TargetAmount).Mul(decimal.NewFromInt(100)).Float64()
		status.Percent = &pct
	}
	return status
}

func advanceDate(date string, rt models.RecurrenceType) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	switch rt {
	case models.RecurrenceMonthly:
		return t.AddDate(0, 1, 0).Format("2006-01-02")
	case models.RecurrenceBiweekly:
		return t.AddDate(0, 0, 14).Format("2006-01-02")
	}
	return date
}
