package services

import (
	"context"
	"math"
	"sort"
	"time"

	"ledger-api/app/internal/models"
)

func NewReportService(repo TransactionRepository, cats CategoryRepository) *ReportService {
	return &ReportService{repo: repo, categories: cats}
}

func (s *ReportService) Summarize(ctx context.Context, accountIDs []string, from, to time.Time) (*models.ReportSummary, error) {
	// Last known balance per account strictly before the period — the starting point.
	carryPerAccount, err := s.repo.GetLastBalancePerAccount(ctx, accountIDs, from)
	if err != nil {
		return nil, err
	}

	txs, err := s.repo.GetByAccountIDsInRange(ctx, accountIDs, from, to)
	if err != nil {
		return nil, err
	}

	allCats, err := s.categories.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	catByID := make(map[string]*models.Category, len(allCats))
	for _, c := range allCats {
		catByID[c.ID] = c
	}

	summary := &models.ReportSummary{
		PeriodStart:    from.Format("2006-01-02"),
		PeriodEnd:      to.Format("2006-01-02"),
		BalanceHistory: []models.DailyBalance{},
		DailyChanges:   []models.DailyChange{},
		ByCategory:     []models.CategorySpend{},
	}

	dailyIncome := map[string]float64{}
	dailyExpenses := map[string]float64{}
	// dailyBalance tracks the summed balance across all accounts for each day
	// that had at least one transaction.
	dailyBalance := map[string]float64{}
	categoryTotals := map[string]*models.CategorySpend{}

	// Seed per-account last balance from carry values so accounts with no
	// in-period transactions still contribute to the running total.
	lastBalPerAccount := make(map[string]float64, len(accountIDs))
	for _, id := range accountIDs {
		lastBalPerAccount[id] = carryPerAccount[id]
	}

	for _, tx := range txs { // sorted date asc by the repo query
		day := tx.Date.Format("2006-01-02")
		amount, _ := tx.Amount.Float64()
		bal, _ := tx.Balance.Float64()

		lastBalPerAccount[tx.AccountID] = bal

		// Sum all accounts' latest balance to get the portfolio total for this day.
		dayTotal := 0.0
		for _, b := range lastBalPerAccount {
			dayTotal += b
		}
		dailyBalance[day] = dayTotal

		switch tx.Type {
		case models.TypeIncome:
			summary.TotalIncome += amount
			dailyIncome[day] += amount
		case models.TypeExpense:
			absAmount := math.Abs(amount) // expenses are stored as negatives; normalise to positive
			summary.TotalExpenses += absAmount
			dailyExpenses[day] += absAmount
			for _, root := range resolveRootCategories(tx.Categories, catByID) {
				if _, ok := categoryTotals[root.ID]; !ok {
					categoryTotals[root.ID] = &models.CategorySpend{
						CategoryID:   root.ID,
						CategoryName: root.Name,
						Color:        root.Color,
					}
				}
				categoryTotals[root.ID].Total += absAmount
			}
		case models.TypeTransfer:
			if amount >= 0 {
				summary.Transfers.IncomingCount++
				summary.Transfers.IncomingTotal += amount
			} else {
				summary.Transfers.OutgoingCount++
				summary.Transfers.OutgoingTotal += math.Abs(amount)
			}
		}
	}

	totalBalance := 0.0
	for _, bal := range lastBalPerAccount {
		totalBalance += bal
	}
	summary.TotalBalance = totalBalance
	summary.PeriodChange = summary.TotalIncome - summary.TotalExpenses

	carryTotal := 0.0
	for _, bal := range carryPerAccount {
		carryTotal += bal
	}
	prevTotal := carryTotal
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		day := d.Format("2006-01-02")
		summary.DailyChanges = append(summary.DailyChanges, models.DailyChange{
			Date:     day,
			Income:   dailyIncome[day],
			Expenses: dailyExpenses[day],
		})
		if total, ok := dailyBalance[day]; ok {
			prevTotal = total
		}
		summary.BalanceHistory = append(summary.BalanceHistory, models.DailyBalance{
			Date:    day,
			Balance: prevTotal,
		})
	}

	for _, cs := range categoryTotals {
		summary.ByCategory = append(summary.ByCategory, *cs)
	}
	sort.Slice(summary.ByCategory, func(i, j int) bool {
		return summary.ByCategory[i].Total > summary.ByCategory[j].Total
	})

	return summary, nil
}

// resolveRootCategories walks each assigned category up one level to its
// parent (root), de-duplicating so a transaction tagged with two children of
// the same parent only counts once against that parent. A transaction with
// multiple distinct roots contributes its full amount to each — spend totals
// intentionally overlap rather than being split arbitrarily between tags.
func resolveRootCategories(cats []*models.Category, catByID map[string]*models.Category) []*models.Category {
	seen := make(map[string]*models.Category, len(cats))
	for _, cat := range cats {
		root := cat
		if cat.ParentID != "" {
			if parent, ok := catByID[cat.ParentID]; ok {
				root = parent
			}
		}
		seen[root.ID] = root
	}

	roots := make([]*models.Category, 0, len(seen))
	for _, root := range seen {
		roots = append(roots, root)
	}
	return roots
}
