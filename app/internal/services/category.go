package services

import (
	"context"
	"strings"

	"ledger-api/app/internal/models"
)

func NewCategoryService(
	categories CategoryRepository,
	rules CategoryRuleRepository,
	txCats TransactionCategoryRepository,
	transactions TransactionRepository,
) *CategoryService {
	return &CategoryService{categories: categories, rules: rules, txCats: txCats, transactions: transactions}
}

func (s *CategoryService) List(ctx context.Context) ([]*models.Category, error) {
	flat, err := s.categories.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	return buildTree(flat), nil
}

func (s *CategoryService) Create(ctx context.Context, c *models.Category) (*models.Category, error) {
	return s.categories.Create(ctx, c)
}

func (s *CategoryService) Update(ctx context.Context, id string, fields map[string]string) (*models.Category, error) {
	return s.categories.Update(ctx, id, fields)
}

func (s *CategoryService) Delete(ctx context.Context, id string) error {
	return s.categories.SoftDelete(ctx, id)
}

func (s *CategoryService) ListRules(ctx context.Context) ([]*models.CategoryRule, error) {
	return s.rules.FindAll(ctx)
}

func (s *CategoryService) CreateRule(ctx context.Context, r *models.CategoryRule) (*models.CategoryRule, error) {
	return s.rules.Create(ctx, r)
}

func (s *CategoryService) DeleteRule(ctx context.Context, id string) error {
	return s.rules.Delete(ctx, id)
}

func (s *CategoryService) SetTransactionCategories(ctx context.Context, transactionID string, categoryIDs []string) error {
	return s.txCats.SetCategories(ctx, transactionID, categoryIDs)
}

// PreviewRule returns transactions that match the rule's pattern (and optional account scope)
// so the caller can show a confirmation before applying.
func (s *CategoryService) PreviewRule(ctx context.Context, ruleID string) ([]*models.Transaction, error) {
	rule, err := s.rules.FindByID(ctx, ruleID)
	if err != nil {
		return nil, err
	}
	if rule == nil {
		return nil, nil
	}
	return s.transactions.FindMatchingPattern(ctx, rule.Pattern, rule.AccountID)
}

// ApplyRule assigns the rule's category to all transactions matching the rule's pattern.
// Existing category assignments on those transactions are preserved (additive, not replacement).
// Returns the number of transactions updated.
func (s *CategoryService) ApplyRule(ctx context.Context, ruleID string) (int, error) {
	rule, err := s.rules.FindByID(ctx, ruleID)
	if err != nil {
		return 0, err
	}
	if rule == nil {
		return 0, nil
	}
	txs, err := s.transactions.FindMatchingPattern(ctx, rule.Pattern, rule.AccountID)
	if err != nil {
		return 0, err
	}
	if len(txs) == 0 {
		return 0, nil
	}
	ids := make([]string, len(txs))
	for i, tx := range txs {
		ids[i] = tx.ID
	}
	if err := s.txCats.AddCategoryBatch(ctx, ids, rule.CategoryID); err != nil {
		return 0, err
	}
	return len(txs), nil
}

func (s *CategoryService) ApplyRules(ctx context.Context, accountID, description string) ([]string, error) {
	rules, err := s.rules.FindByAccountID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	desc := strings.ToLower(description)
	for _, rule := range rules {
		if strings.Contains(desc, strings.ToLower(rule.Pattern)) {
			return []string{rule.CategoryID}, nil
		}
	}
	return nil, nil
}

func buildTree(flat []*models.Category) []*models.Category {
	byID := make(map[string]*models.Category, len(flat))
	for _, c := range flat {
		byID[c.ID] = c
	}
	var roots []*models.Category
	for _, c := range flat {
		if c.ParentID == "" {
			roots = append(roots, c)
		} else if parent, ok := byID[c.ParentID]; ok {
			parent.Children = append(parent.Children, c)
		} else {
			roots = append(roots, c)
		}
	}
	return roots
}
