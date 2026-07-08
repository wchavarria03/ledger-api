package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"ledger-api/app/internal/models"
)

func NewTransferService(accounts AccountRepository, transactions TransactionRepository, transfers TransferRepository) *TransferService {
	return &TransferService{accounts: accounts, transactions: transactions, transfers: transfers}
}

// CreateTransfer records money moving from one account to another as a
// linked pair of transactions (a debit on the source, a credit on the
// destination), plus a row in the transfers table connecting them so the
// existing reconciliation logic recognizes them as a real transfer.
//
// Cross-currency transfers are rejected for now — both accounts must use
// the same currency as the request. BCCR exchange-rate support is tracked
// separately in docs/FUTURE_ENHANCEMENTS.md.
func (s *TransferService) CreateTransfer(ctx context.Context, input models.TransferInput) (*models.TransferResult, error) {
	if input.FromAccountID == "" || input.ToAccountID == "" {
		return nil, fmt.Errorf("from_account_id and to_account_id are required")
	}
	if input.FromAccountID == input.ToAccountID {
		return nil, fmt.Errorf("from_account_id and to_account_id must be different")
	}
	if input.Amount.LessThanOrEqual(decimal.Zero) {
		return nil, fmt.Errorf("amount must be positive")
	}

	fromAccount, err := s.accounts.FindByID(ctx, input.FromAccountID)
	if err != nil {
		return nil, fmt.Errorf("lookup from account: %w", err)
	}
	if fromAccount == nil {
		return nil, fmt.Errorf("from account not found")
	}

	toAccount, err := s.accounts.FindByID(ctx, input.ToAccountID)
	if err != nil {
		return nil, fmt.Errorf("lookup to account: %w", err)
	}
	if toAccount == nil {
		return nil, fmt.Errorf("to account not found")
	}

	if fromAccount.Currency != input.Currency || toAccount.Currency != input.Currency {
		return nil, fmt.Errorf(
			"cross-currency transfers are not supported yet: both accounts must use %s (from=%s, to=%s)",
			input.Currency, fromAccount.Currency, toAccount.Currency,
		)
	}

	description := input.Description
	if description == "" {
		description = fmt.Sprintf("Transfer: %s -> %s", fromAccount.Name, toAccount.Name)
	}

	fromTx, err := s.transactions.Create(ctx, &models.Transaction{
		AccountID:   input.FromAccountID,
		Date:        input.Date,
		Description: description,
		Amount:      input.Amount.Neg(),
		Type:        models.TypeTransfer,
		Currency:    input.Currency,
	})
	if err != nil {
		return nil, fmt.Errorf("create outgoing transaction: %w", err)
	}

	toTx, err := s.transactions.Create(ctx, &models.Transaction{
		AccountID:   input.ToAccountID,
		Date:        input.Date,
		Description: description,
		Amount:      input.Amount,
		Type:        models.TypeTransfer,
		Currency:    input.Currency,
	})
	if err != nil {
		// Compensate: remove the leg we already wrote so the source
		// account doesn't end up silently short with no matching credit.
		if delErr := s.transactions.Delete(ctx, fromTx.ID); delErr != nil {
			return nil, fmt.Errorf(
				"create incoming transaction: %w (also failed to roll back outgoing leg %s: %v)",
				err, fromTx.ID, delErr,
			)
		}
		return nil, fmt.Errorf("create incoming transaction: %w", err)
	}

	transfer, err := s.transfers.Create(ctx, fromTx.ID, toTx.ID, nil, "calculated")
	if err != nil {
		return nil, fmt.Errorf("link transfer: %w", err)
	}

	if err := s.transactions.SetTransferID(ctx, fromTx.ID, transfer.ID); err != nil {
		return nil, fmt.Errorf("link outgoing transaction to transfer: %w", err)
	}
	if err := s.transactions.SetTransferID(ctx, toTx.ID, transfer.ID); err != nil {
		return nil, fmt.Errorf("link incoming transaction to transfer: %w", err)
	}
	fromTx.TransferID = transfer.ID
	toTx.TransferID = transfer.ID

	return &models.TransferResult{
		Transfer:        *transfer,
		FromTransaction: *fromTx,
		ToTransaction:   *toTx,
	}, nil
}

// MatchForPeriod fetches all accounts and their transactions in the given date
// range, runs the transfer matching algorithm, and returns the matched pairs.
func (s *TransferService) MatchForPeriod(ctx context.Context, from, to time.Time) ([]models.TransferMatch, error) {
	accounts, err := s.accounts.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}

	ids := make([]string, len(accounts))
	for i, a := range accounts {
		ids[i] = a.ID
	}

	allTxs, err := s.transactions.GetByAccountIDsInRange(ctx, ids, from, to)
	if err != nil {
		return nil, fmt.Errorf("fetch transactions: %w", err)
	}

	// Group transactions by account to build synthetic statements.
	txsByAccount := make(map[string][]*models.Transaction, len(accounts))
	for _, tx := range allTxs {
		txsByAccount[tx.AccountID] = append(txsByAccount[tx.AccountID], tx)
	}

	stmts := make([]*models.Statement, 0, len(accounts))
	for _, acc := range accounts {
		txs := txsByAccount[acc.ID]
		flat := make([]models.Transaction, len(txs))
		for i, t := range txs {
			flat[i] = *t
		}
		stmts = append(stmts, &models.Statement{
			AccountNumber: acc.AccountNumber,
			ShortNumber:   acc.ShortNumber,
			Transactions:  flat,
		})
	}

	pairs := s.Match(stmts)

	// Flatten all transactions for index lookup (same iteration order as Match).
	var flat []models.Transaction
	for _, stmt := range stmts {
		flat = append(flat, stmt.Transactions...)
	}

	matches := make([]models.TransferMatch, 0, len(pairs))
	for _, p := range pairs {
		if p.a < len(flat) && p.b < len(flat) {
			matches = append(matches, models.TransferMatch{
				From:       flat[p.a],
				To:         flat[p.b],
				Confidence: p.confidence,
			})
		}
	}

	return matches, nil
}

// ReconcileForPeriod finds unlinked transfer-typed transactions across all
// accounts in [from, to], matches them, and persists links for high-confidence
// pairs (tier 1: same reference, tier 2: short-number cross-reference).
// Tier-3-only pairs (same date + amount, no corroborating evidence) are
// skipped and left for manual review.
// Returns the number of transfer pairs successfully linked.
func (s *TransferService) ReconcileForPeriod(ctx context.Context, from, to time.Time) (int, error) {
	accounts, err := s.accounts.FindAll(ctx)
	if err != nil {
		return 0, fmt.Errorf("list accounts: %w", err)
	}

	ids := make([]string, len(accounts))
	for i, a := range accounts {
		ids[i] = a.ID
	}

	allTxs, err := s.transactions.GetByAccountIDsInRange(ctx, ids, from, to)
	if err != nil {
		return 0, fmt.Errorf("fetch transactions: %w", err)
	}

	// Index accounts by ID for short-number lookup.
	shortByAcct := make(map[string]string, len(accounts))
	acctNumByID := make(map[string]string, len(accounts))
	for _, acc := range accounts {
		shortByAcct[acc.ID] = acc.ShortNumber
		acctNumByID[acc.ID] = acc.AccountNumber
	}

	// Group only unlinked, transfer-typed transactions by account.
	txsByAccount := make(map[string][]*models.Transaction, len(accounts))
	for _, tx := range allTxs {
		if tx.Type != models.TypeTransfer || tx.TransferID != "" {
			continue
		}
		txsByAccount[tx.AccountID] = append(txsByAccount[tx.AccountID], tx)
	}

	// Build synthetic statements from accounts that have qualifying transactions.
	stmts := make([]*models.Statement, 0, len(accounts))
	for _, acc := range accounts {
		txs := txsByAccount[acc.ID]
		if len(txs) == 0 {
			continue
		}
		flat := make([]models.Transaction, len(txs))
		for i, t := range txs {
			flat[i] = *t
		}
		stmts = append(stmts, &models.Statement{
			AccountNumber: acc.AccountNumber,
			ShortNumber:   acc.ShortNumber,
			Transactions:  flat,
		})
	}

	if len(stmts) == 0 {
		return 0, nil
	}

	pairs := s.Match(stmts)

	// Flatten in the same iteration order Match() used, so indices align.
	var flat []models.Transaction
	for _, stmt := range stmts {
		flat = append(flat, stmt.Transactions...)
	}

	linked := 0
	for _, pair := range pairs {
		// Skip tier-3 (amount + date only) — requires user confirmation.
		if pair.confidence == models.MatchByAmountDate {
			continue
		}
		if pair.a >= len(flat) || pair.b >= len(flat) {
			continue
		}
		fromTx := flat[pair.a]
		toTx := flat[pair.b]

		transfer, err := s.transfers.Create(ctx, fromTx.ID, toTx.ID, nil, "auto")
		if err != nil {
			// Non-fatal: one bad pair must not abort the whole reconciliation.
			continue
		}
		_ = s.transactions.SetTransferID(ctx, fromTx.ID, transfer.ID)
		_ = s.transactions.SetTransferID(ctx, toTx.ID, transfer.ID)
		linked++
	}

	return linked, nil
}

// matchPair holds the indices of a matched pair within the flattened
// transaction list, plus the confidence tier that identified it.
type matchPair struct {
	a, b       int
	confidence models.MatchConfidence
}

// Match identifies transfer pairs across statements using a 3-tier priority:
//  1. Same Reference across accounts (strongest)
//  2. Description contains counterpart's ShortNumber (TEF A/DE patterns)
//  3. Same date + same absolute amount + same currency (weakest, needs user confirmation)
//
// Returns pairs of (a, b) indices into the flattened transaction list, with
// the confidence tier that produced the match.
func (s *TransferService) Match(statements []*models.Statement) []matchPair {
	type indexed struct {
		stmtIdx int
		txIdx   int
		tx      models.Transaction
	}

	var all []indexed
	for si, stmt := range statements {
		for ti, tx := range stmt.Transactions {
			all = append(all, indexed{si, ti, tx})
		}
	}

	shortNumbers := make(map[int]string, len(statements))
	for i, stmt := range statements {
		shortNumbers[i] = stmt.ShortNumber
	}

	matched := make(map[int]bool)
	var pairs []matchPair

	for i, a := range all {
		if matched[i] {
			continue
		}
		for j, b := range all {
			if j <= i || matched[j] || a.stmtIdx == b.stmtIdx {
				continue
			}
			if confidence, ok := isTransferPair(a.tx, b.tx, shortNumbers[a.stmtIdx], shortNumbers[b.stmtIdx]); ok {
				matched[i] = true
				matched[j] = true
				pairs = append(pairs, matchPair{a: i, b: j, confidence: confidence})
				break
			}
		}
	}

	return pairs
}

func isTransferPair(a, b models.Transaction, shortA, shortB string) (models.MatchConfidence, bool) {
	// Tier 1: same non-empty reference
	if a.Reference != "" && a.Reference == b.Reference {
		return models.MatchByReference, true
	}

	// Tier 2: TEF description contains counterpart's short number
	if shortB != "" && strings.Contains(a.Description, shortB) {
		return models.MatchByShortNumber, true
	}
	if shortA != "" && strings.Contains(b.Description, shortA) {
		return models.MatchByShortNumber, true
	}

	// Tier 3: same date + same currency + same absolute amount (opposite signs)
	if a.Date.Equal(b.Date) && a.Currency == b.Currency && a.Amount.Add(b.Amount).IsZero() {
		return models.MatchByAmountDate, true
	}

	return "", false
}
