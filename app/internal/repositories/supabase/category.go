package supabase

import (
	"context"
	"net/url"
	"time"

	"ledger-api/app/internal/databases"
	"ledger-api/app/internal/models"
)

func NewCategoryRepository(client *databases.SupabaseClient) *CategoryRepository {
	return &CategoryRepository{client: client}
}

func (r *CategoryRepository) FindAll(ctx context.Context) ([]*models.Category, error) {
	return databases.Get[[]*models.Category](ctx, r.client, "/rest/v1/categories", url.Values{
		"order": []string{"name.asc"},
	})
}

func (r *CategoryRepository) FindByID(ctx context.Context, id string) (*models.Category, error) {
	results, err := databases.Get[[]*models.Category](ctx, r.client, "/rest/v1/categories", url.Values{
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

func (r *CategoryRepository) Create(ctx context.Context, c *models.Category) (*models.Category, error) {
	results, err := databases.Post[[]*models.Category](ctx, r.client, "/rest/v1/categories", c, "return=representation")
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	return results[0], nil
}

func (r *CategoryRepository) Update(ctx context.Context, id string, fields map[string]string) (*models.Category, error) {
	results, err := databases.Patch[[]*models.Category](ctx, r.client,
		"/rest/v1/categories?id=eq."+id,
		fields, "return=representation")
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	return results[0], nil
}

func (r *CategoryRepository) SoftDelete(ctx context.Context, id string) error {
	_, err := databases.Patch[[]*models.Category](ctx, r.client,
		"/rest/v1/categories?id=eq."+id,
		map[string]string{"deleted_at": time.Now().UTC().Format(time.RFC3339)},
		"")
	return err
}

// ── CategoryRuleRepository ────────────────────────────────────────────────────

func NewCategoryRuleRepository(client *databases.SupabaseClient) *CategoryRuleRepository {
	return &CategoryRuleRepository{client: client}
}

func (r *CategoryRuleRepository) FindAll(ctx context.Context) ([]*models.CategoryRule, error) {
	return databases.Get[[]*models.CategoryRule](ctx, r.client, "/rest/v1/category_rules", url.Values{
		"order": []string{"priority.desc"},
	})
}

func (r *CategoryRuleRepository) FindByID(ctx context.Context, id string) (*models.CategoryRule, error) {
	results, err := databases.Get[[]*models.CategoryRule](ctx, r.client, "/rest/v1/category_rules", url.Values{
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

func (r *CategoryRuleRepository) FindByAccountID(ctx context.Context, accountID string) ([]*models.CategoryRule, error) {
	return databases.Get[[]*models.CategoryRule](ctx, r.client, "/rest/v1/category_rules", url.Values{
		"or":    []string{"(account_id.is.null,account_id.eq." + accountID + ")"},
		"order": []string{"priority.desc"},
	})
}

func (r *CategoryRuleRepository) Create(ctx context.Context, rule *models.CategoryRule) (*models.CategoryRule, error) {
	results, err := databases.Post[[]*models.CategoryRule](ctx, r.client, "/rest/v1/category_rules", rule, "return=representation")
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	return results[0], nil
}

func (r *CategoryRuleRepository) Delete(ctx context.Context, id string) error {
	return databases.Delete(ctx, r.client, "/rest/v1/category_rules?id=eq."+id)
}

// ── TransactionCategoryRepository ────────────────────────────────────────────

func NewTransactionCategoryRepository(client *databases.SupabaseClient) *TransactionCategoryRepository {
	return &TransactionCategoryRepository{client: client}
}

func (r *TransactionCategoryRepository) SetCategories(ctx context.Context, transactionID string, categoryIDs []string) error {
	if err := databases.Delete(ctx, r.client, "/rest/v1/transaction_categories?transaction_id=eq."+transactionID); err != nil {
		return err
	}
	if len(categoryIDs) == 0 {
		return nil
	}
	type row struct {
		TransactionID string `json:"transaction_id"`
		CategoryID    string `json:"category_id"`
	}
	rows := make([]row, len(categoryIDs))
	for i, id := range categoryIDs {
		rows[i] = row{TransactionID: transactionID, CategoryID: id}
	}
	_, err := databases.Post[struct{}](ctx, r.client, "/rest/v1/transaction_categories", rows, "")
	return err
}

// AddCategoryBatch inserts (transaction_id, category_id) rows without replacing existing assignments.
// Conflicts on (transaction_id, category_id) are silently ignored.
func (r *TransactionCategoryRepository) AddCategoryBatch(ctx context.Context, txIDs []string, categoryID string) error {
	if len(txIDs) == 0 {
		return nil
	}
	type row struct {
		TransactionID string `json:"transaction_id"`
		CategoryID    string `json:"category_id"`
	}
	rows := make([]row, len(txIDs))
	for i, id := range txIDs {
		rows[i] = row{TransactionID: id, CategoryID: categoryID}
	}
	_, err := databases.Post[struct{}](ctx, r.client,
		"/rest/v1/transaction_categories?on_conflict=transaction_id,category_id",
		rows, "resolution=ignore-duplicates")
	return err
}
