package supabase

import (
	"context"
	"net/url"

	"ledger-api/app/internal/databases"
	"ledger-api/app/internal/models"
)

func NewAccountRepository(client *databases.SupabaseClient) *AccountRepository {
	return &AccountRepository{client: client}
}

func (r *AccountRepository) FindAll(ctx context.Context) ([]*models.Account, error) {
	return databases.Get[[]*models.Account](ctx, r.client, "/rest/v1/accounts", url.Values{
		"order": []string{"created_at.desc"},
	})
}

func (r *AccountRepository) FindByID(ctx context.Context, id string) (*models.Account, error) {
	results, err := databases.Get[[]*models.Account](ctx, r.client, "/rest/v1/accounts", url.Values{
		"id":    []string{"eq." + id},
		"limit": []string{"1"},
	})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	return results[0], nil
}

func (r *AccountRepository) FindByAccountNumber(ctx context.Context, number string) (*models.Account, error) {
	results, err := databases.Get[[]*models.Account](ctx, r.client, "/rest/v1/accounts", url.Values{
		"account_number": []string{"eq." + number},
		"limit":          []string{"1"},
	})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	return results[0], nil
}

func (r *AccountRepository) Update(ctx context.Context, id string, fields map[string]any) (*models.Account, error) {
	results, err := databases.Patch[[]*models.Account](ctx, r.client,
		"/rest/v1/accounts?id=eq."+id,
		fields,
		"return=representation")
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	return results[0], nil
}

func (r *AccountRepository) Upsert(ctx context.Context, a *models.Account) (*models.Account, error) {
	results, err := databases.Post[[]*models.Account](ctx, r.client, "/rest/v1/accounts", a,
		"resolution=merge-duplicates,return=representation")
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return a, nil
	}
	return results[0], nil
}
