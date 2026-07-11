package services

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"ledger-api/app/internal/auth"
	"ledger-api/app/internal/models"
)

func NewBudgetService(budgets BudgetRepository, accounts AccountRepository, transactions TransactionRepository) *BudgetService {
	return &BudgetService{
		budgets:      budgets,
		accounts:     accounts,
		transactions: transactions,
	}
}

// List returns all budgets enriched with spending data for the given month (YYYY-MM).
func (s *BudgetService) List(ctx context.Context, month string) ([]models.BudgetStatus, error) {
	t, err := time.Parse("2006-01", month)
	if err != nil {
		return nil, fmt.Errorf("invalid month format, expected YYYY-MM: %w", err)
	}
	from := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	// Day 0 of next month resolves to the last day of the current month.
	to := time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, time.UTC)

	budgets, err := s.budgets.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list budgets: %w", err)
	}
	if len(budgets) == 0 {
		return []models.BudgetStatus{}, nil
	}

	budgetIDs := make([]string, len(budgets))
	for i, b := range budgets {
		budgetIDs[i] = b.ID
	}
	acks, err := s.budgets.ListAcknowledgments(ctx, budgetIDs, month)
	if err != nil {
		return nil, fmt.Errorf("list acknowledgments: %w", err)
	}
	ackedSet := make(map[string]bool, len(acks))
	for _, a := range acks {
		ackedSet[a.BudgetID] = true
	}

	allAccounts, err := s.accounts.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}

	// Group account IDs by currency so we fetch transactions once per currency.
	idsByCurrency := make(map[string][]string)
	for _, a := range allAccounts {
		idsByCurrency[a.Currency] = append(idsByCurrency[a.Currency], a.ID)
	}

	txsByCurrency := make(map[string][]*models.Transaction)
	for currency, ids := range idsByCurrency {
		txs, err := s.transactions.GetByAccountIDsInRange(ctx, ids, from, to)
		if err != nil {
			return nil, fmt.Errorf("fetch transactions for %s: %w", currency, err)
		}
		txsByCurrency[currency] = txs
	}

	statuses := make([]models.BudgetStatus, len(budgets))
	for i, b := range budgets {
		spent := computeSpent(txsByCurrency[b.Currency], b.CategoryID)
		remaining := b.Amount.Sub(spent)
		var percent float64
		if b.Amount.IsPositive() {
			pct, _ := spent.Div(b.Amount).Mul(decimal.NewFromInt(100)).Float64()
			percent = pct
		}
		statuses[i] = models.BudgetStatus{
			Budget:       *b,
			Spent:        spent,
			Remaining:    remaining,
			Percent:      percent,
			Status:       statusFromPercent(percent),
			Acknowledged: ackedSet[b.ID],
		}
	}
	return statuses, nil
}

func (s *BudgetService) Create(ctx context.Context, input models.BudgetInput) (*models.Budget, error) {
	userID := auth.UserIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("no authenticated user")
	}
	if !input.Amount.IsPositive() {
		return nil, fmt.Errorf("amount must be positive")
	}
	input.UserID = userID
	return s.budgets.Create(ctx, input)
}

func (s *BudgetService) Update(ctx context.Context, id string, amount decimal.Decimal) (*models.Budget, error) {
	if !amount.IsPositive() {
		return nil, fmt.Errorf("amount must be positive")
	}
	return s.budgets.Update(ctx, id, amount)
}

func (s *BudgetService) Delete(ctx context.Context, id string) error {
	return s.budgets.Delete(ctx, id)
}

// Acknowledge records the user's decision on an underspent budget for a month.
// If action is "moved", a transfer_id linking the movement must be provided.
func (s *BudgetService) Acknowledge(ctx context.Context, budgetID, month, action string, transferID *string) (*models.BudgetAcknowledgment, error) {
	if action != "kept" && action != "moved" {
		return nil, fmt.Errorf("action must be 'kept' or 'moved'")
	}
	if action == "moved" && transferID == nil {
		return nil, fmt.Errorf("transfer_id is required when action is 'moved'")
	}
	return s.budgets.Acknowledge(ctx, budgetID, month, action, transferID)
}

// computeSpent sums the absolute amount of all expense transactions that match categoryID.
func computeSpent(txs []*models.Transaction, categoryID string) decimal.Decimal {
	spent := decimal.Zero
	for _, tx := range txs {
		if tx.Type != models.TypeExpense {
			continue
		}
		for _, c := range tx.Categories {
			if c.ID == categoryID {
				spent = spent.Add(tx.Amount.Abs())
				break
			}
		}
	}
	return spent
}

func statusFromPercent(percent float64) string {
	switch {
	case percent >= 100:
		return "exceeded"
	case percent >= 80:
		return "warning"
	default:
		return "on_track"
	}
}
