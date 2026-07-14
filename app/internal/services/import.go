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
	reminderSvc *ReminderService,
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
		reminderSvc:    reminderSvc,
		userID:         userID,
	}
}

func (s *ImportService) Import(ctx context.Context, stmt *models.Statement, bankName string) error {
	_, _, _, err := s.doImport(ctx, stmt, bankName, nil)
	return err
}

func (s *ImportService) ImportWithSummary(ctx context.Context, stmt *models.Statement, bankName string, catOverrides map[int][]string) (*models.ImportSummary, error) {
	results, linked, reminderMatches, err := s.doImport(ctx, stmt, bankName, catOverrides)
	if err != nil {
		return nil, err
	}
	bank := bankName
	if idx := strings.Index(bankName, "/"); idx != -1 {
		bank = bankName[:idx]
	}

	accounts := make([]models.ImportedAccountSummary, len(results))
	for i, r := range results {
		accounts[i] = models.ImportedAccountSummary{
			AccountName:   r.account.Name,
			AccountNumber: r.account.AccountNumber,
			AccountIsNew:  r.isNew,
			Currency:      r.account.Currency,
			Bank:          bank,
			ImportedCount: r.count,
		}
	}

	return &models.ImportSummary{
		Accounts:             accounts,
		LinkedTransfersCount: linked,
		ReminderMatches:      reminderMatches,
	}, nil
}

// CheckOverlap reports how many transactions already exist in the accounts
// this statement would import into, across all currencies it carries.
func (s *ImportService) CheckOverlap(ctx context.Context, stmt *models.Statement) (int, error) {
	if len(stmt.Transactions) == 0 {
		return 0, nil
	}

	total := 0
	for _, g := range groupByCurrency(stmt) {
		acc, err := s.accounts.FindByAccountNumber(ctx, g.accountNumber)
		if err != nil {
			return 0, fmt.Errorf("lookup account: %w", err)
		}
		if acc == nil {
			continue
		}

		from := g.transactions[0].Date
		to := g.transactions[len(g.transactions)-1].Date
		existing, err := s.transactions.GetByAccountIDsInRange(ctx, []string{acc.ID}, from, to)
		if err != nil {
			return 0, fmt.Errorf("check existing: %w", err)
		}
		total += len(existing)
	}

	return total, nil
}

// importedAccount is one currency-group's outcome from doImport.
type importedAccount struct {
	account *models.Account
	isNew   bool
	count   int
}

// currencyGroup is a statement's transactions split by currency, plus the
// account_number that currency resolves to. Almost always a single group
// (accountNumber == stmt.AccountNumber); mixed-currency statements (e.g. BAC
// credit cards billing CRC and USD on one physical card) produce one group
// per currency, since an Account always holds a single currency.
type currencyGroup struct {
	currency      string
	accountNumber string
	transactions  []models.Transaction
	origIndexes   []int
}

func groupByCurrency(stmt *models.Statement) []currencyGroup {
	order := make([]string, 0, 2)
	byCurrency := make(map[string]*currencyGroup, 2)
	for i, tx := range stmt.Transactions {
		g, ok := byCurrency[tx.Currency]
		if !ok {
			g = &currencyGroup{currency: tx.Currency}
			byCurrency[tx.Currency] = g
			order = append(order, tx.Currency)
		}
		g.transactions = append(g.transactions, tx)
		g.origIndexes = append(g.origIndexes, i)
	}

	multi := len(order) > 1
	groups := make([]currencyGroup, len(order))
	for i, currency := range order {
		g := byCurrency[currency]
		g.accountNumber = stmt.AccountNumber
		if multi {
			g.accountNumber = stmt.AccountNumber + ":" + currency
		}
		groups[i] = *g
	}
	return groups
}

func (s *ImportService) doImport(ctx context.Context, stmt *models.Statement, bankName string, catOverrides map[int][]string) ([]importedAccount, int, []models.ReminderMatch, error) {
	bank := bankName
	if idx := strings.Index(bankName, "/"); idx != -1 {
		bank = bankName[:idx]
	}

	// Prefer the user ID from the JWT context; fall back to the static value for CLI use.
	userID := auth.UserIDFromContext(ctx)
	if userID == "" {
		userID = s.userID
	}

	groups := groupByCurrency(stmt)
	multi := len(groups) > 1

	var results []importedAccount
	var reminderMatches []models.ReminderMatch

	for _, g := range groups {
		acc, err := s.accounts.FindByAccountNumber(ctx, g.accountNumber)
		if err != nil {
			return nil, 0, nil, fmt.Errorf("lookup account: %w", err)
		}

		isNew := acc == nil
		if isNew {
			name := strings.ToUpper(bank)
			if len(stmt.AccountNumber) >= 4 {
				name = strings.ToUpper(bank) + " - ****" + stmt.AccountNumber[len(stmt.AccountNumber)-4:]
			}
			if multi {
				name += " (" + g.currency + ")"
			}

			acc, err = s.accounts.Upsert(ctx, &models.Account{
				AccountNumber: g.accountNumber,
				ShortNumber:   stmt.ShortNumber,
				BankName:      bank,
				Name:          name,
				Currency:      g.currency,
				UserID:        userID,
			})
			if err != nil {
				return nil, 0, nil, fmt.Errorf("upsert account: %w", err)
			}
		}

		txs, err := s.classifier.Apply(ctx, bank, g.transactions)
		if err != nil {
			return nil, 0, nil, fmt.Errorf("classify transactions: %w", err)
		}

		if err := s.transactions.UpsertBatch(ctx, acc.ID, stmt.SourceFile, txs); err != nil {
			return nil, 0, nil, fmt.Errorf("upsert transactions: %w", err)
		}

		// Apply user-supplied category overrides before auto-categorization so
		// autoCategorize skips these transactions (it checks len(Categories) > 0).
		// catOverrides is keyed by index into the original (pre-split)
		// stmt.Transactions, so translate via each row's origIndexes.
		if len(catOverrides) > 0 {
			var groupOverrides []categoryOverride
			for gi, origIdx := range g.origIndexes {
				if catIDs, ok := catOverrides[origIdx]; ok {
					groupOverrides = append(groupOverrides, categoryOverride{tx: g.transactions[gi], categoryIDs: catIDs})
				}
			}
			if len(groupOverrides) > 0 {
				s.applyCategoryOverrides(ctx, acc.ID, groupOverrides)
			}
		}

		// Auto-categorize newly imported transactions using category rules.
		// Errors here are non-fatal — the import already succeeded.
		s.autoCategorize(ctx, acc.ID, &models.Statement{Transactions: g.transactions})

		// Surface candidate matches between resolved-but-unconfirmed reminders
		// and the newly imported transactions, for the user to confirm.
		// Best-effort — same pattern as autoCategorize.
		reminderMatches = append(reminderMatches, s.matchReminders(ctx, acc.ID, &models.Statement{Transactions: g.transactions})...)

		results = append(results, importedAccount{account: acc, isNew: isNew, count: len(g.transactions)})
	}

	// Auto-reconcile transfer pairs across all accounts for this period.
	// Errors here are non-fatal — same best-effort pattern as autoCategorize.
	// Runs once for the whole statement — it scans across all accounts anyway.
	linked := s.autoReconcile(ctx, stmt)

	return results, linked, reminderMatches, nil
}

// matchReminders looks up transactions imported in this statement's date
// range and checks them against resolved-but-unconfirmed reminders on the
// same account.
func (s *ImportService) matchReminders(ctx context.Context, accountID string, stmt *models.Statement) []models.ReminderMatch {
	if s.reminderSvc == nil || len(stmt.Transactions) == 0 {
		return nil
	}
	from := stmt.Transactions[0].Date
	to := stmt.Transactions[len(stmt.Transactions)-1].Date
	stored, err := s.transactions.GetByAccountIDsInRange(ctx, []string{accountID}, from, to)
	if err != nil {
		return nil
	}
	matches, err := s.reminderSvc.MatchCandidates(ctx, accountID, stored)
	if err != nil {
		return nil
	}
	return matches
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

// categoryOverride pairs a parsed transaction (as it appeared in the source
// statement) with the category IDs the user assigned it during import review.
type categoryOverride struct {
	tx          models.Transaction
	categoryIDs []string
}

// applyCategoryOverrides sets explicit categories for transactions the user
// corrected during the import review. Matches stored transactions by
// (date, reference, amount) — the same unique key used by UpsertBatch.
func (s *ImportService) applyCategoryOverrides(ctx context.Context, accountID string, overrides []categoryOverride) {
	if s.txCategories == nil || len(overrides) == 0 {
		return
	}
	from, to := overrides[0].tx.Date, overrides[0].tx.Date
	for _, o := range overrides {
		if o.tx.Date.Before(from) {
			from = o.tx.Date
		}
		if o.tx.Date.After(to) {
			to = o.tx.Date
		}
	}
	stored, err := s.transactions.GetByAccountIDsInRange(ctx, []string{accountID}, from, to)
	if err != nil {
		return
	}

	type key struct{ date, ref, amount string }
	byKey := make(map[key]string, len(stored))
	for _, tx := range stored {
		byKey[key{tx.Date.Format("2006-01-02"), tx.Reference, tx.Amount.String()}] = tx.ID
	}

	for _, o := range overrides {
		k := key{o.tx.Date.Format("2006-01-02"), o.tx.Reference, o.tx.Amount.String()}
		if id, ok := byKey[k]; ok && id != "" {
			_ = s.txCategories.SetCategories(ctx, id, o.categoryIDs)
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
