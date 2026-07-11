package supabase

import (
	"context"
	"net/url"
	"strings"

	"github.com/shopspring/decimal"

	"ledger-api/app/internal/databases"
	"ledger-api/app/internal/models"
)

func NewEnvelopeRepository(client *databases.SupabaseClient) *EnvelopeRepository {
	return &EnvelopeRepository{client: client}
}

func (r *EnvelopeRepository) List(ctx context.Context) ([]*models.Envelope, error) {
	return databases.Get[[]*models.Envelope](ctx, r.client, "/rest/v1/envelopes", url.Values{
		"order": []string{"created_at.asc"},
	})
}

func (r *EnvelopeRepository) ListByAccountID(ctx context.Context, accountID string) ([]*models.Envelope, error) {
	return databases.Get[[]*models.Envelope](ctx, r.client, "/rest/v1/envelopes", url.Values{
		"account_id": []string{"eq." + accountID},
		"order":      []string{"created_at.asc"},
	})
}

func (r *EnvelopeRepository) FindByID(ctx context.Context, id string) (*models.Envelope, error) {
	rows, err := databases.Get[[]*models.Envelope](ctx, r.client, "/rest/v1/envelopes", url.Values{
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

func (r *EnvelopeRepository) Create(ctx context.Context, input models.EnvelopeInput) (*models.Envelope, error) {
	rows, err := databases.Post[[]*models.Envelope](ctx, r.client,
		"/rest/v1/envelopes",
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

func (r *EnvelopeRepository) Update(ctx context.Context, id string, fields map[string]any) (*models.Envelope, error) {
	rows, err := databases.Patch[[]*models.Envelope](ctx, r.client,
		"/rest/v1/envelopes?id=eq."+id,
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

func (r *EnvelopeRepository) Delete(ctx context.Context, id string) error {
	return databases.Delete(ctx, r.client, "/rest/v1/envelopes?id=eq."+id)
}

func (r *EnvelopeRepository) Contribute(ctx context.Context, envelopeID string, input models.ContributionInput) (*models.EnvelopeContribution, error) {
	type row struct {
		EnvelopeID string          `json:"envelope_id"`
		Amount     decimal.Decimal `json:"amount"`
		Note       string          `json:"note,omitempty"`
		Date       string          `json:"date"`
	}
	body := row{
		EnvelopeID: envelopeID,
		Amount:     input.Amount,
		Note:       input.Note,
		Date:       input.Date,
	}
	rows, err := databases.Post[[]*models.EnvelopeContribution](ctx, r.client,
		"/rest/v1/envelope_contributions",
		body,
		"return=representation")
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

// GetBalances returns the current balance (sum of contributions) for each envelope ID.
func (r *EnvelopeRepository) GetBalances(ctx context.Context, envelopeIDs []string) (map[string]decimal.Decimal, error) {
	result := make(map[string]decimal.Decimal, len(envelopeIDs))
	for _, id := range envelopeIDs {
		result[id] = decimal.Zero
	}
	if len(envelopeIDs) == 0 {
		return result, nil
	}

	type contribRow struct {
		EnvelopeID string          `json:"envelope_id"`
		Amount     decimal.Decimal `json:"amount"`
	}
	rows, err := databases.Get[[]*contribRow](ctx, r.client, "/rest/v1/envelope_contributions", url.Values{
		"envelope_id": []string{"in.(" + strings.Join(envelopeIDs, ",") + ")"},
		"select":      []string{"envelope_id,amount"},
	})
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.EnvelopeID] = result[row.EnvelopeID].Add(row.Amount)
	}
	return result, nil
}

// SetNextContributionDate advances the schedule after a recurring contribution is applied.
func (r *EnvelopeRepository) SetNextContributionDate(ctx context.Context, id, date string) error {
	_, err := databases.Patch[struct{}](ctx, r.client,
		"/rest/v1/envelopes?id=eq."+id,
		map[string]any{"next_contribution_date": date},
		"return=minimal")
	return err
}
