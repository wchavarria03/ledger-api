package supabase

import (
	"context"
	"net/url"

	"ledger-api/app/internal/databases"
	"ledger-api/app/internal/models"
)

func NewSalaryProfileRepository(client *databases.SupabaseClient) *SalaryProfileRepository {
	return &SalaryProfileRepository{client: client}
}

func (r *SalaryProfileRepository) FindByUserID(ctx context.Context, userID string) (*models.SalaryProfile, error) {
	results, err := databases.Get[[]*models.SalaryProfile](ctx, r.client, "/rest/v1/salary_profiles", url.Values{
		"user_id": []string{"eq." + userID},
		"limit":   []string{"1"},
	})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	return results[0], nil
}

// Upsert creates or replaces the caller's salary profile. One profile per
// user_id — conflicts resolve on that column rather than the primary key.
func (r *SalaryProfileRepository) Upsert(ctx context.Context, p *models.SalaryProfile) (*models.SalaryProfile, error) {
	results, err := databases.Post[[]*models.SalaryProfile](ctx, r.client,
		"/rest/v1/salary_profiles?on_conflict=user_id", p,
		"resolution=merge-duplicates,return=representation")
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return p, nil
	}
	return results[0], nil
}
