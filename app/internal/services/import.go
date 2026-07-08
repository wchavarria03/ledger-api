package services

import (
	"context"
	"fmt"
	"strings"

	"ledger-api/app/internal/auth"
	"ledger-api/app/internal/models"
)

func NewImportService(
	accounts AccountRepository,
	transactions TransactionRepository,
	classifier *ClassificationService,
	categoryRules CategoryRuleRepository,
	txCategories TransactionCategoryRepository,
	ruleExceptions AccountRuleExceptionRepository,
	transferSvc *TransferService,
	userID string,
) *ImportService {
	return &ImportService{
		accounts:       accounts,
		transactions:   transactions,
		classifier:     classifier,
		categoryRules:  categoryRules,
		txCategories:   txCategories,
		ruleExceptions: ruleExceptions,
		transferSvc:    transferSvc,
		userID:         userID,
	}
}

func (s *ImportService) Import(ctx context.Context, stmt *models.Statement, bankName string) error {
	_, _, err := s.doImport(ctx, stmt, bankName, nil)
	return err
}

func (s *ImportService) ImportWithSummary(ctx context.Context, stmt *models.Statement, bankName string, catOverrides map[int][]string) (*models.ImportSummary, error) {
	acc, linked, err := s.doImport(ctx, stmt, bankName, catOverrides)
	if err != nil {
		return nil, err
	}
	bank := bankName
	if idx := strings.Index(bankName, "/"); idx != -1 {
		bank = bankName[:idx]
	}
	return &models.ImportSummary{
		AccountName:          acc.Name,
		AccountNumber:        stmt.AccountNumber,
		Currency:             acc.Currency,
		Bank:                 bank,
		ImportedCount:        len(stmt.Transactions),
		LinkedTransfersCount: linked,
	}, nil
}

func (s *ImportService) CheckOverlap(ctx context.Context, stmt *models.Statement) (int, error) {
	if len(stmt.Transactions) == 0 {
		return 0, nil
	}

	acc, err := s.accounts.FindByAccountNumber(ctx, stmt.AccountNumber)
	if err != nil {
		return 0, fmt.Errorf("lookup account: %w", err)
	}
	if acc == nil {
		return 0, nil
	}

	from := stmt.Transactions[0].Date
	to := stmt.Transactions[len(stmt.Transactions)-1].Date

	existing, err := s.transactions.GetByAccountIDsInRange(ctx, []string{acc.ID}, from, to)
	if err != nil {
		return 0, fmt.Errorf("check existing: %w", err)
	}

	return len(existing), nil
}

func (s *ImportService) doImport(ctx context.Context, stmt *models.Statement, bankName string, catOverrides map[int][]string) (*models.Account, int, error) {
	bank := bankName
	if idx := strings.Index(bankName, "/"); idx != -1 {
		bank = bankName[:idx]
	}

	// Prefer the user ID from the JWT context; fall back to the static value for CLI use.
	userID := auth.UserIDFromContext(ctx)
	if userID == "" {
		userID = s.userID
	}

	acc, err := s.accounts.FindByAccountNumber(ctx, stmt.AccountNumber)
	if err != nil {
		return nil, 0, fmt.Errorf("lookup account: %w", err)
	}

	if acc == nil {
		currency := "CRC"
		if len(stmt.Transactions) > 0 {
			currency = stmt.Transactions[0].Currency
		}

		name := strings.ToUpper(bank)
		if len(stmt.AccountNumber) >= 4 {
			name = strings.ToUpper(bank) + " - ****" + stmt.AccountNumber[len(stmt.AccountNumber)-4:]
		}

		acc, err = s.accounts.Upsert(ctx, &models.Account{
			AccountNumber: stmt.AccountNumber,
			ShortNumber:   stmt.ShortNumber,
			BankName:      bank,
			Name:          name,
			Currency:      currency,
			UserID:        userID,
		})
		if err != nil {
			return nil, 0, fmt.Errorf("upsert account: %w", err)
		}
	}

	txs, err := s.classifier.Apply(ctx, bank, stmt.Transactions)
	if err != nil {
		return nil, 0, fmt.Errorf("classify transactions: %w", err)
	}

	if err := s.transactions.UpsertBatch(ctx, acc.ID, stmt.SourceFile, txs); err != nil {
		return nil, 0, fmt.Errorf("upsert transactions: %w", err)
	}

	// Apply user-supplied category overrides before auto-categorization so
	// autoCategorize skips these transactions (it checks len(Categories) > 0).
	if len(catOverrides) > 0 {
		s.applyCategoryOverrides(ctx, acc.ID, stmt.Transactions, catOverrides)
	}

	// Auto-categorize newly imported transactions using category rules.
	// Errors here are non-fatal — the import already succeeded.
	s.autoCategorize(ctx, acc.ID, stmt)

	// Auto-reconcile transfer pairs across all accounts for this period.
	// Errors here are non-fatal — same best-effort pattern as autoCategorize.
	linked := s.autoReconcile(ctx, stmt)

	return acc, linked, nil
}

// autoReconcile runs transfer reconciliation across all accounts for the
// statement's date range and returns the number of pairs linked.
func (s *ImportService) autoReconcile(ctx context.Context, stmt *models.Statement) int {
	if s.transferSvc == nil || len(stmt.Transactions) == 0 {
		return 0
	}
	from := stmt.Transactions[0].Date
	to := stmt.Transactions[len(stmt.Transactions)-1].Date
	linked, err := s.transferSvc.ReconcileForPeriod(ctx, from, to)
	if err != nil {
		return 0
	}
	return linked
}

// autoCategorize applies category rules to uncategorized transactions in the statement period.
func (s *ImportService) autoCategorize(ctx context.Context, accountID string, stmt *models.Statement) {
	if s.categoryRules == nil || s.txCategories == nil || len(stmt.Transactions) == 0 {
		return
	}

	rules, err := s.categoryRules.FindByAccountID(ctx, accountID)
	if err != nil {
		return
	}
	if len(rules) == 0 {
		return
	}

	// Build set of disabled global rule IDs for this account.
	disabledIDs := map[string]bool{}
	if s.ruleExceptions != nil {
		if ids, err := s.ruleExceptions.FindByAccount(ctx, accountID); err == nil {
			for _, id := range ids {
				disabledIDs[id] = true
			}
		}
	}

	from := stmt.Transactions[0].Date
	to := stmt.Transactions[len(stmt.Transactions)-1].Date
	stored, err := s.transactions.GetByAccountIDsInRange(ctx, []string{accountID}, from, to)
	if err != nil {
		return
	}

	for _, tx := range stored {
		if len(tx.Categories) > 0 {
			continue // already categorized — don't overwrite manual work
		}
		catID := matchCategoryRule(tx, rules, disabledIDs)
		if catID == "" {
			continue
		}
		// ignore error — categorization is best-effort
		_ = s.txCategories.SetCategories(ctx, tx.ID, []string{catID})
	}
}

// applyCategoryOverrides sets explicit categories for transactions the user
// corrected during the import review. Matches stored transactions by
// (date, reference, amount) — the same unique key used by UpsertBatch.
func (s *ImportService) applyCategoryOverrides(ctx context.Context, accountID string, txs []models.Transaction, overrides map[int][]string) {
	if s.txCategories == nil || len(txs) == 0 {
		return
	}
	from := txs[0].Date
	to := txs[len(txs)-1].Date
	stored, err := s.transactions.GetByAccountIDsInRange(ctx, []string{accountID}, from, to)
	if err != nil {
		return
	}

	type key struct{ date, ref, amount string }
	byKey := make(map[key]string, len(stored))
	for _, tx := range stored {
		byKey[key{tx.Date.Format("2006-01-02"), tx.Reference, tx.Amount.String()}] = tx.ID
	}

	for idx, catIDs := range overrides {
		if idx < 0 || idx >= len(txs) {
			continue
		}
		tx := txs[idx]
		k := key{tx.Date.Format("2006-01-02"), tx.Reference, tx.Amount.String()}
		if id, ok := byKey[k]; ok && id != "" {
			_ = s.txCategories.SetCategories(ctx, id, catIDs)
		}
	}
}

// matchCategoryRule returns the category_id of the best matching rule for tx.
// Account-specific rules take priority over global rules at the same priority level.
func matchCategoryRule(tx *models.Transaction, rules []*models.CategoryRule, disabledIDs map[string]bool) string {
	desc := strings.ToUpper(tx.Description)
	var best *models.CategoryRule
	for _, r := range rules {
		// Skip disabled global rules.
		if r.AccountID == "" && disabledIDs[r.ID] {
			continue
		}
		if !strings.Contains(desc, strings.ToUpper(r.Pattern)) {
			continue
		}
		if best == nil {
			best = r
			continue
		}
		// Account-specific rule beats global rule at the same priority.
		if r.Priority > best.Priority || (r.Priority == best.Priority && r.AccountID != "" && best.AccountID == "") {
			best = r
		}
	}
	if best == nil {
		return ""
	}
	return best.CategoryID
}
