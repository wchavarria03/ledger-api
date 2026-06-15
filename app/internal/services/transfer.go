package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ledger-api/app/internal/models"
)

func NewTransferService(accounts AccountRepository, transactions TransactionRepository) *TransferService {
	return &TransferService{accounts: accounts, transactions: transactions}
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

	// Flatten all transactions for index lookup.
	var flat []models.Transaction
	for _, stmt := range stmts {
		flat = append(flat, stmt.Transactions...)
	}

	matches := make([]models.TransferMatch, 0, len(pairs))
	for _, p := range pairs {
		if p[0] < len(flat) && p[1] < len(flat) {
			matches = append(matches, models.TransferMatch{
				From: flat[p[0]],
				To:   flat[p[1]],
			})
		}
	}

	return matches, nil
}

// Match identifies transfer pairs across statements using a 3-tier priority:
//  1. Same Reference across accounts (strongest)
//  2. Description contains counterpart's ShortNumber (TEF A/DE patterns)
//  3. Same date + same absolute amount (weakest, needs user confirmation)
//
// Returns pairs of (outIndex, inIndex) into the flattened transaction list.
func (s *TransferService) Match(statements []*models.Statement) [][2]int {
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
	var pairs [][2]int

	for i, a := range all {
		if matched[i] {
			continue
		}
		for j, b := range all {
			if j <= i || matched[j] || a.stmtIdx == b.stmtIdx {
				continue
			}
			if isTransferPair(a.tx, b.tx, shortNumbers[a.stmtIdx], shortNumbers[b.stmtIdx]) {
				matched[i] = true
				matched[j] = true
				pairs = append(pairs, [2]int{i, j})
				break
			}
		}
	}

	return pairs
}

func isTransferPair(a, b models.Transaction, shortA, shortB string) bool {
	// Tier 1: same non-empty reference
	if a.Reference != "" && a.Reference == b.Reference {
		return true
	}

	// Tier 2: TEF description contains counterpart's short number
	if shortB != "" && strings.Contains(a.Description, shortB) {
		return true
	}
	if shortA != "" && strings.Contains(b.Description, shortA) {
		return true
	}

	// Tier 3: same date + same absolute amount (opposite signs)
	if a.Date.Equal(b.Date) && a.Amount.Add(b.Amount).IsZero() {
		return true
	}

	return false
}
