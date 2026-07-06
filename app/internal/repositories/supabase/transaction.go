package supabase

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"ledger-api/app/internal/databases"
	"ledger-api/app/internal/models"
)

// transactionRow is the write shape — used for UpsertBatch only.
type transactionRow struct {
	ID          string          `json:"id,omitempty"`
	AccountID   string          `json:"account_id"`
	Date        string          `json:"date"`
	Reference   string          `json:"reference,omitempty"`
	Code        string          `json:"code,omitempty"`
	Type        string          `json:"type"`
	Description string          `json:"description"`
	Amount      decimal.Decimal `json:"amount"`
	Balance     decimal.Decimal `json:"balance"`
	Currency    string          `json:"currency"`
	SourceFile  string          `json:"source_file,omitempty"`
}

// transactionRowFull is the read shape — includes embedded category join.
type transactionRowFull struct {
	ID                    string             `json:"id,omitempty"`
	AccountID             string             `json:"account_id"`
	Date                  string             `json:"date"`
	Reference             string             `json:"reference,omitempty"`
	Code                  string             `json:"code,omitempty"`
	Type                  string             `json:"type"`
	Description           string             `json:"description"`
	Amount                decimal.Decimal    `json:"amount"`
	Balance               decimal.Decimal    `json:"balance"`
	Currency              string             `json:"currency"`
	TransactionCategories []txCategoryEmbed  `json:"transaction_categories"`
}

type txCategoryEmbed struct {
	Category *models.Category `json:"categories"`
}

func NewTransactionRepository(client *databases.SupabaseClient) *TransactionRepository {
	return &TransactionRepository{client: client}
}

func (r *TransactionRepository) Create(ctx context.Context, tx *models.Transaction) (*models.Transaction, error) {
	balances, err := r.GetCurrentBalances(ctx, []string{tx.AccountID})
	if err != nil {
		return nil, err
	}
	lastBal := decimal.NewFromFloat(balances[tx.AccountID])

	row := transactionRow{
		AccountID:   tx.AccountID,
		Date:        tx.Date.Format("2006-01-02"),
		Reference:   tx.Reference,
		Type:        string(tx.Type),
		Description: tx.Description,
		Amount:      tx.Amount,
		Balance:     lastBal.Add(tx.Amount),
		Currency:    tx.Currency,
	}

	results, err := databases.Post[[]*transactionRow](ctx, r.client, "/rest/v1/transactions", row, "return=representation")
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("create transaction: no result returned")
	}
	res := results[0]
	date, _ := time.Parse("2006-01-02", res.Date)
	return &models.Transaction{
		ID:          res.ID,
		AccountID:   res.AccountID,
		Date:        date,
		Reference:   res.Reference,
		Type:        models.TransactionType(res.Type),
		Description: res.Description,
		Amount:      res.Amount,
		Balance:     res.Balance,
		Currency:    res.Currency,
	}, nil
}

func (r *TransactionRepository) GetByAccountID(ctx context.Context, accountID string) ([]*models.Transaction, error) {
	rows, err := databases.Get[[]*transactionRowFull](ctx, r.client, "/rest/v1/transactions", url.Values{
		"account_id": []string{"eq." + accountID},
		"select":     []string{"*,transaction_categories(categories(id,name,color,parent_id))"},
		"order":      []string{"date.desc"},
	})
	if err != nil {
		return nil, err
	}
	txs := make([]*models.Transaction, len(rows))
	for i, row := range rows {
		date, _ := time.Parse("2006-01-02", row.Date)
		cats := make([]*models.Category, 0, len(row.TransactionCategories))
		for _, tc := range row.TransactionCategories {
			if tc.Category != nil {
				cats = append(cats, tc.Category)
			}
		}
		txs[i] = &models.Transaction{
			ID:          row.ID,
			AccountID:   row.AccountID,
			Date:        date,
			Reference:   row.Reference,
			Code:        row.Code,
			Type:        models.TransactionType(row.Type),
			Description: row.Description,
			Amount:      row.Amount,
			Balance:     row.Balance,
			Currency:    row.Currency,
			Categories:  cats,
		}
	}
	return txs, nil
}

type countRow struct {
	Count int `json:"count"`
}

func txBaseParams(accountID string, filter models.TxFilter) url.Values {
	params := url.Values{
		"account_id": []string{"eq." + accountID},
	}
	if filter.Search != "" {
		params.Set("description", "ilike.*"+filter.Search+"*")
	}
	if filter.Type != "" {
		params.Set("type", "eq."+filter.Type)
	}
	var dateFilters []string
	if filter.From != "" {
		dateFilters = append(dateFilters, "gte."+filter.From)
	}
	if filter.To != "" {
		dateFilters = append(dateFilters, "lte."+filter.To)
	}
	if len(dateFilters) > 0 {
		params["date"] = dateFilters
	}
	return params
}

func (r *TransactionRepository) ListFiltered(ctx context.Context, accountID string, filter models.TxFilter) ([]*models.Transaction, int, error) {
	base := txBaseParams(accountID, filter)

	// Count query
	countParams := url.Values{}
	for k, v := range base {
		countParams[k] = v
	}
	countParams.Set("select", "count")
	counts, err := databases.Get[[]*countRow](ctx, r.client, "/rest/v1/transactions", countParams)
	if err != nil {
		return nil, 0, err
	}
	total := 0
	if len(counts) > 0 {
		total = counts[0].Count
	}

	// Data query
	page := filter.Page
	if page < 1 {
		page = 1
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := (page - 1) * limit

	dataParams := url.Values{}
	for k, v := range base {
		dataParams[k] = v
	}
	dataParams.Set("select", "*,transaction_categories(categories(id,name,color,parent_id))")
	dataParams.Set("order", "date.desc")
	dataParams.Set("limit", strconv.Itoa(limit))
	dataParams.Set("offset", strconv.Itoa(offset))

	rows, err := databases.Get[[]*transactionRowFull](ctx, r.client, "/rest/v1/transactions", dataParams)
	if err != nil {
		return nil, 0, err
	}
	txs := make([]*models.Transaction, len(rows))
	for i, row := range rows {
		date, _ := time.Parse("2006-01-02", row.Date)
		cats := make([]*models.Category, 0, len(row.TransactionCategories))
		for _, tc := range row.TransactionCategories {
			if tc.Category != nil {
				cats = append(cats, tc.Category)
			}
		}
		txs[i] = &models.Transaction{
			ID:          row.ID,
			AccountID:   row.AccountID,
			Date:        date,
			Reference:   row.Reference,
			Code:        row.Code,
			Type:        models.TransactionType(row.Type),
			Description: row.Description,
			Amount:      row.Amount,
			Balance:     row.Balance,
			Currency:    row.Currency,
			Categories:  cats,
		}
	}
	return txs, total, nil
}

func (r *TransactionRepository) GetByAccountIDsInRange(ctx context.Context, accountIDs []string, from, to time.Time) ([]*models.Transaction, error) {
	rows, err := databases.Get[[]*transactionRowFull](ctx, r.client, "/rest/v1/transactions", url.Values{
		"account_id": []string{"in.(" + strings.Join(accountIDs, ",") + ")"},
		"date":       []string{"gte." + from.Format("2006-01-02"), "lte." + to.Format("2006-01-02")},
		"select":     []string{"*,transaction_categories(categories(id,name,color,parent_id))"},
		"order":      []string{"date.asc"},
	})
	if err != nil {
		return nil, err
	}
	txs := make([]*models.Transaction, len(rows))
	for i, row := range rows {
		date, _ := time.Parse("2006-01-02", row.Date)
		cats := make([]*models.Category, 0, len(row.TransactionCategories))
		for _, tc := range row.TransactionCategories {
			if tc.Category != nil {
				cats = append(cats, tc.Category)
			}
		}
		txs[i] = &models.Transaction{
			ID:          row.ID,
			AccountID:   row.AccountID,
			Date:        date,
			Reference:   row.Reference,
			Code:        row.Code,
			Type:        models.TransactionType(row.Type),
			Description: row.Description,
			Amount:      row.Amount,
			Balance:     row.Balance,
			Currency:    row.Currency,
			Categories:  cats,
		}
	}
	return txs, nil
}

// GetCurrentBalances returns the most recent balance for each account.
// Accounts with no transactions are omitted from the result.
func (r *TransactionRepository) GetCurrentBalances(ctx context.Context, accountIDs []string) (map[string]float64, error) {
	result := make(map[string]float64, len(accountIDs))
	for _, id := range accountIDs {
		rows, err := databases.Get[[]*transactionRowFull](ctx, r.client, "/rest/v1/transactions", url.Values{
			"account_id": []string{"eq." + id},
			"select":     []string{"account_id,balance"},
			"order":      []string{"date.desc"},
			"limit":      []string{"1"},
		})
		if err != nil {
			return nil, err
		}
		if len(rows) > 0 {
			bal, _ := rows[0].Balance.Float64()
			result[id] = bal
		}
	}
	return result, nil
}

// GetLastBalancePerAccount returns the last known balance for each account
// strictly before the given date. Accounts with no prior transactions are omitted.
func (r *TransactionRepository) GetLastBalancePerAccount(ctx context.Context, accountIDs []string, before time.Time) (map[string]float64, error) {
	result := make(map[string]float64, len(accountIDs))
	for _, id := range accountIDs {
		rows, err := databases.Get[[]*transactionRowFull](ctx, r.client, "/rest/v1/transactions", url.Values{
			"account_id": []string{"eq." + id},
			"date":       []string{"lt." + before.Format("2006-01-02")},
			"select":     []string{"account_id,balance"},
			"order":      []string{"date.desc"},
			"limit":      []string{"1"},
		})
		if err != nil {
			return nil, err
		}
		if len(rows) > 0 {
			bal, _ := rows[0].Balance.Float64()
			result[id] = bal
		}
	}
	return result, nil
}

// Delete removes a transaction by ID. Used to compensate for a partially
// failed transfer (the counterpart leg couldn't be written).
func (r *TransactionRepository) Delete(ctx context.Context, id string) error {
	return databases.Delete(ctx, r.client, "/rest/v1/transactions?id=eq."+id)
}

// SetTransferID stamps a transaction with the transfer row that links it
// to its counterpart leg.
func (r *TransactionRepository) SetTransferID(ctx context.Context, txID, transferID string) error {
	_, err := databases.Patch[struct{}](ctx, r.client,
		"/rest/v1/transactions?id=eq."+txID,
		map[string]any{"transfer_id": transferID},
		"return=minimal")
	return err
}

func (r *TransactionRepository) UpsertBatch(ctx context.Context, accountID string, sourceFile string, txs []models.Transaction) error {
	rows := make([]transactionRow, len(txs))
	for i, tx := range txs {
		rows[i] = transactionRow{
			AccountID:   accountID,
			Date:        tx.Date.Format("2006-01-02"),
			Reference:   tx.Reference,
			Code:        tx.Code,
			Type:        string(tx.Type),
			Description: tx.Description,
			Amount:      tx.Amount,
			Balance:     tx.Balance,
			Currency:    tx.Currency,
			SourceFile:  sourceFile,
		}
	}
	_, err := databases.Post[struct{}](ctx, r.client,
		"/rest/v1/transactions?on_conflict=account_id,date,reference,amount",
		rows, "resolution=ignore-duplicates")
	return err
}
