package supabase

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"ledger-api/app/internal/databases"
	"ledger-api/app/internal/models"
)

// budgetRow is the read shape, including the embedded category join.
type budgetRow struct {
	ID         string          `json:"id"`
	UserID     string          `json:"user_id"`
	CategoryID string          `json:"category_id"`
	Currency   string          `json:"currency"`
	Amount     decimal.Decimal `json:"amount"`
	CreatedAt  time.Time       `json:"created_at"`
	Category   *struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	} `json:"categories"`
}

func (r budgetRow) toModel() *models.Budget {
	b := &models.Budget{
		ID:         r.ID,
		UserID:     r.UserID,
		CategoryID: r.CategoryID,
		Currency:   r.Currency,
		Amount:     r.Amount,
		CreatedAt:  r.CreatedAt,
	}
	if r.Category != nil {
		b.CategoryName = r.Category.Name
		b.CategoryColor = r.Category.Color
	}
	return b
}

func NewBudgetRepository(client *databases.SupabaseClient) *BudgetRepository {
	return &BudgetRepository{client: client}
}

func (r *BudgetRepository) List(ctx context.Context) ([]*models.Budget, error) {
	rows, err := databases.Get[[]*budgetRow](ctx, r.client, "/rest/v1/budgets", url.Values{
		"select": []string{"*,categories(name,color)"},
		"order":  []string{"created_at.asc"},
	})
	if err != nil {
		return nil, err
	}
	result := make([]*models.Budget, len(rows))
	for i, row := range rows {
		result[i] = row.toModel()
	}
	return result, nil
}

func (r *BudgetRepository) Create(ctx context.Context, input models.BudgetInput) (*models.Budget, error) {
	rows, err := databases.Post[[]*budgetRow](ctx, r.client,
		"/rest/v1/budgets?select=*,categories(name,color)",
		input,
		"return=representation")
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0].toModel(), nil
}

func (r *BudgetRepository) FindByID(ctx context.Context, id string) (*models.Budget, error) {
	rows, err := databases.Get[[]*budgetRow](ctx, r.client, "/rest/v1/budgets", url.Values{
		"id":     []string{"eq." + id},
		"select": []string{"*,categories(name,color)"},
		"limit":  []string{"1"},
	})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0].toModel(), nil
}

func (r *BudgetRepository) Update(ctx context.Context, id string, amount decimal.Decimal) (*models.Budget, error) {
	rows, err := databases.Patch[[]*budgetRow](ctx, r.client,
		"/rest/v1/budgets?id=eq."+id+"&select=*,categories(name,color)",
		map[string]any{"amount": amount},
		"return=representation")
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0].toModel(), nil
}

func (r *BudgetRepository) Delete(ctx context.Context, id string) error {
	return databases.Delete(ctx, r.client, "/rest/v1/budgets?id=eq."+id)
}

func (r *BudgetRepository) Acknowledge(ctx context.Context, budgetID, month, action string, transferID *string) (*models.BudgetAcknowledgment, error) {
	type ackInsert struct {
		BudgetID   string  `json:"budget_id"`
		Month      string  `json:"month"`
		Action     string  `json:"action"`
		TransferID *string `json:"transfer_id,omitempty"`
	}
	body := ackInsert{
		BudgetID:   budgetID,
		Month:      month,
		Action:     action,
		TransferID: transferID,
	}
	rows, err := databases.Post[[]*models.BudgetAcknowledgment](ctx, r.client,
		"/rest/v1/budget_acknowledgments?on_conflict=budget_id,month",
		body,
		"resolution=merge-duplicates,return=representation")
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

func (r *BudgetRepository) ListAcknowledgments(ctx context.Context, budgetIDs []string, month string) ([]*models.BudgetAcknowledgment, error) {
	if len(budgetIDs) == 0 {
		return nil, nil
	}
	return databases.Get[[]*models.BudgetAcknowledgment](ctx, r.client, "/rest/v1/budget_acknowledgments", url.Values{
		"budget_id": []string{"in.(" + strings.Join(budgetIDs, ",") + ")"},
		"month":     []string{"eq." + month},
	})
}
