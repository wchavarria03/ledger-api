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
	ImportSeq   *int            `json:"import_seq,omitempty"`
}

// transactionRowFull is the read shape — includes embedded category and transfer joins.
type transactionRowFull struct {
	ID                    string            `json:"id,omitempty"`
	AccountID             string            `json:"account_id"`
	Date                  string            `json:"date"`
	Reference             string            `json:"reference,omitempty"`
	Code                  string            `json:"code,omitempty"`
	Type                  string            `json:"type"`
	Description           string            `json:"description"`
	Amount                decimal.Decimal   `json:"amount"`
	Balance               decimal.Decimal   `json:"balance"`
	Currency              string            `json:"currency"`
	TransferID            string            `json:"transfer_id,omitempty"`
	Transfer              *transferEmbed    `json:"transfers,omitempty"`
	TransactionCategories []txCategoryEmbed `json:"transaction_categories"`
}

type txCategoryEmbed struct {
	Category *models.Category `json:"categories"`
}

// transferEmbed holds the two transaction IDs from the linked transfers row.
type transferEmbed struct {
	FromTxID string `json:"from_tx_id"`
	ToTxID   string `json:"to_tx_id"`
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

func (r *TransactionRepository) GetByID(ctx context.Context, id string) (*models.Transaction, error) {
	rows, err := databases.Get[[]*transactionRowFull](ctx, r.client, "/rest/v1/transactions", url.Values{
		"id":     []string{"eq." + id},
		"select": []string{"*,transaction_categories(categories(id,name,color,parent_id))"},
		"limit":  []string{"1"},
	})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	row := rows[0]
	date, _ := time.Parse("2006-01-02", row.Date)
	cats := make([]*models.Category, 0, len(row.TransactionCategories))
	for _, tc := range row.TransactionCategories {
		if tc.Category != nil {
			cats = append(cats, tc.Category)
		}
	}
	return &models.Transaction{
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
		TransferID:  row.TransferID,
		Categories:  cats,
	}, nil
}

func (r *TransactionRepository) GetByAccountID(ctx context.Context, accountID string) ([]*models.Transaction, error) {
	rows, err := databases.Get[[]*transactionRowFull](ctx, r.client, "/rest/v1/transactions", url.Values{
		"account_id": []string{"eq." + accountID},
		"select":     []string{"*,transaction_categories(categories(id,name,color,parent_id))"},
		"order":      []string{"date.desc,import_seq.desc"},
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
			TransferID:  row.TransferID,
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
	dataParams.Set("select", "*,transaction_categories(categories(id,name,color,parent_id)),transfers!transfer_id(from_tx_id,to_tx_id)")
	dataParams.Set("order", "date.desc,import_seq.desc")
	dataParams.Set("limit", strconv.Itoa(limit))
	dataParams.Set("offset", strconv.Itoa(offset))

	rows, err := databases.Get[[]*transactionRowFull](ctx, r.client, "/rest/v1/transactions", dataParams)
	if err != nil {
		return nil, 0, err
	}

	counterpartNames, err := r.resolveCounterpartNames(ctx, rows)
	if err != nil {
		// Non-fatal: degrade gracefully — transactions still load without counterpart names.
		counterpartNames = map[string]string{}
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
			ID:                     row.ID,
			AccountID:              row.AccountID,
			Date:                   date,
			Reference:              row.Reference,
			Code:                   row.Code,
			Type:                   models.TransactionType(row.Type),
			Description:            row.Description,
			Amount:                 row.Amount,
			Balance:                row.Balance,
			Currency:               row.Currency,
			TransferID:             row.TransferID,
			CounterpartAccountName: counterpartNames[row.ID],
			Categories:             cats,
		}
	}
	return txs, total, nil
}

// resolveCounterpartNames fetches the account name of the counterpart transaction
// for every linked transfer row in the page. Returns a map of tx.ID → counterpart account name.
func (r *TransactionRepository) resolveCounterpartNames(ctx context.Context, rows []*transactionRowFull) (map[string]string, error) {
	// Map this tx's ID → counterpart tx ID for any linked transfers.
	counterpartTxID := make(map[string]string) // this-tx-id → counterpart-tx-id
	for _, row := range rows {
		if row.Transfer == nil {
			continue
		}
		if row.Transfer.FromTxID == row.ID {
			counterpartTxID[row.ID] = row.Transfer.ToTxID
		} else {
			counterpartTxID[row.ID] = row.Transfer.FromTxID
		}
	}
	if len(counterpartTxID) == 0 {
		return map[string]string{}, nil
	}

	// Collect unique counterpart tx IDs.
	cpIDs := make([]string, 0, len(counterpartTxID))
	seen := make(map[string]bool)
	for _, cpID := range counterpartTxID {
		if !seen[cpID] {
			cpIDs = append(cpIDs, cpID)
			seen[cpID] = true
		}
	}

	// Fetch counterpart transactions joined to their account.
	type cpRow struct {
		ID      string `json:"id"`
		Account *struct {
			Name  string `json:"name"`
			Alias string `json:"alias"`
		} `json:"accounts"`
	}
	cpRows, err := databases.Get[[]*cpRow](ctx, r.client, "/rest/v1/transactions", url.Values{
		"id":     []string{"in.(" + strings.Join(cpIDs, ",") + ")"},
		"select": []string{"id,accounts!account_id(name,alias)"},
	})
	if err != nil {
		return nil, err
	}

	// Build counterpart-tx-id → account display name.
	nameByTxID := make(map[string]string, len(cpRows))
	for _, cp := range cpRows {
		if cp.Account == nil {
			continue
		}
		name := cp.Account.Alias
		if name == "" {
			name = cp.Account.Name
		}
		nameByTxID[cp.ID] = name
	}

	// Map this-tx-id → account name.
	result := make(map[string]string, len(counterpartTxID))
	for thisTxID, cpID := range counterpartTxID {
		if name, ok := nameByTxID[cpID]; ok {
			result[thisTxID] = name
		}
	}
	return result, nil
}

func (r *TransactionRepository) GetByAccountIDsInRange(ctx context.Context, accountIDs []string, from, to time.Time) ([]*models.Transaction, error) {
	rows, err := databases.Get[[]*transactionRowFull](ctx, r.client, "/rest/v1/transactions", url.Values{
		"account_id": []string{"in.(" + strings.Join(accountIDs, ",") + ")"},
		"date":       []string{"gte." + from.Format("2006-01-02"), "lte." + to.Format("2006-01-02")},
		"select":     []string{"*,transaction_categories(categories(id,name,color,parent_id))"},
		"order":      []string{"date.asc,import_seq.asc"},
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
			TransferID:  row.TransferID,
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
			"order":      []string{"date.desc,import_seq.desc"},
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
			"order":      []string{"date.desc,import_seq.desc"},
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

// FindMatchingPattern returns transactions whose description matches the given pattern (case-insensitive).
// If accountID is non-empty, results are scoped to that account.
func (r *TransactionRepository) FindMatchingPattern(ctx context.Context, pattern, accountID string) ([]*models.Transaction, error) {
	params := url.Values{
		"description": []string{"ilike.*" + pattern + "*"},
		"select":      []string{"id,account_id,date,reference,code,type,description,amount,balance,currency"},
		"order":       []string{"date.desc,import_seq.desc"},
		"limit":       []string{"200"},
	}
	if accountID != "" {
		params.Set("account_id", "eq."+accountID)
	}
	rows, err := databases.Get[[]*transactionRowFull](ctx, r.client, "/rest/v1/transactions", params)
	if err != nil {
		return nil, err
	}
	txs := make([]*models.Transaction, len(rows))
	for i, row := range rows {
		date, _ := time.Parse("2006-01-02", row.Date)
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
		}
	}
	return txs, nil
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
			ImportSeq:   tx.ImportSeq,
		}
	}
	_, err := databases.Post[struct{}](ctx, r.client,
		"/rest/v1/transactions?on_conflict=account_id,date,reference,amount",
		rows, "resolution=ignore-duplicates")
	return err
}
