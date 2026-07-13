package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"ledger-api/app/internal/models"
)

var (
	ErrFromTxNotFound = errors.New("from_tx_id not found")
	ErrToTxNotFound   = errors.New("to_tx_id not found")
	ErrFromTxLinked   = errors.New("from_tx_id is already linked to a transfer")
	ErrToTxLinked     = errors.New("to_tx_id is already linked to a transfer")
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

// LinkTransactions links two already-existing transactions as the two legs of a
// transfer. Unlike CreateTransfer, no new transaction rows are created — the
// caller supplies the IDs of transactions that came from an imported statement.
func (s *TransferService) LinkTransactions(ctx context.Context, fromTxID, toTxID string) (*models.TransferResult, error) {
	if fromTxID == "" || toTxID == "" {
		return nil, fmt.Errorf("from_tx_id and to_tx_id are required")
	}
	if fromTxID == toTxID {
		return nil, fmt.Errorf("from_tx_id and to_tx_id must be different")
	}

	fromTx, err := s.transactions.GetByID(ctx, fromTxID)
	if err != nil {
		return nil, fmt.Errorf("lookup from transaction: %w", err)
	}
	if fromTx == nil {
		return nil, ErrFromTxNotFound
	}

	toTx, err := s.transactions.GetByID(ctx, toTxID)
	if err != nil {
		return nil, fmt.Errorf("lookup to transaction: %w", err)
	}
	if toTx == nil {
		return nil, ErrToTxNotFound
	}

	if fromTx.TransferID != "" {
		return nil, ErrFromTxLinked
	}
	if toTx.TransferID != "" {
		return nil, ErrToTxLinked
	}

	var exchangeRate *float64
	if fromTx.Currency == toTx.Currency {
		if !fromTx.Amount.Add(toTx.Amount).IsZero() {
			return nil, fmt.Errorf(
				"amounts must net to zero: %s + %s = %s",
				fromTx.Amount, toTx.Amount, fromTx.Amount.Add(toTx.Amount),
			)
		}
	} else {
		// Cross-currency link (e.g. an informal currency exchange with someone
		// else): one leg must be a debit and the other a credit, and we derive
		// the implied rate from the two amounts rather than requiring them to
		// net to zero (they're in different currencies).
		if fromTx.Amount.Sign() == toTx.Amount.Sign() {
			return nil, fmt.Errorf(
				"cross-currency transfer legs must have opposite signs: from_tx=%s, to_tx=%s",
				fromTx.Amount, toTx.Amount,
			)
		}
		if fromTx.Amount.IsZero() || toTx.Amount.IsZero() {
			return nil, fmt.Errorf("transfer legs cannot have a zero amount")
		}
		rate, _ := toTx.Amount.Abs().Div(fromTx.Amount.Abs()).Float64()
		exchangeRate = &rate
	}

	transfer, err := s.transfers.Create(ctx, fromTxID, toTxID, exchangeRate, "manual")
	if err != nil {
		return nil, fmt.Errorf("link transfer: %w", err)
	}

	if err := s.transactions.SetTransferID(ctx, fromTxID, transfer.ID); err != nil {
		return nil, fmt.Errorf("link from transaction: %w", err)
	}
	if err := s.transactions.SetTransferID(ctx, toTxID, transfer.ID); err != nil {
		return nil, fmt.Errorf("link to transaction: %w", err)
	}
	fromTx.TransferID = transfer.ID
	toTx.TransferID = transfer.ID

	return &models.TransferResult{
		Transfer:        *transfer,
		FromTransaction: *fromTx,
		ToTransaction:   *toTx,
	}, nil
}

// UpdateTransactionType corrects a transaction's type after import (e.g. a
// payment to an external party that the bank/parser miscoded as a transfer).
// If the transaction is linked to a transfer and the new type is no longer
// "transfer", the link is torn down: the transfer row is deleted and both
// legs' transfer_id are cleared, freeing the counterpart leg for re-matching.
func (s *TransferService) UpdateTransactionType(ctx context.Context, txID string, newType models.TransactionType) (*models.Transaction, error) {
	switch newType {
	case models.TypeExpense, models.TypeIncome, models.TypeTransfer:
	default:
		return nil, fmt.Errorf("type must be expense, income, or transfer")
	}

	tx, err := s.transactions.GetByID(ctx, txID)
	if err != nil {
		return nil, fmt.Errorf("lookup transaction: %w", err)
	}
	if tx == nil {
		return nil, nil
	}

	if tx.TransferID != "" && newType != models.TypeTransfer {
		transfer, err := s.transfers.GetByID(ctx, tx.TransferID)
		if err != nil {
			return nil, fmt.Errorf("lookup transfer: %w", err)
		}
		if transfer != nil {
			counterpartID := transfer.ToTxID
			if counterpartID == txID {
				counterpartID = transfer.FromTxID
			}
			if err := s.transfers.Delete(ctx, transfer.ID); err != nil {
				return nil, fmt.Errorf("delete transfer link: %w", err)
			}
			if err := s.transactions.ClearTransferID(ctx, counterpartID); err != nil {
				return nil, fmt.Errorf("unlink counterpart transaction: %w", err)
			}
			if err := s.transactions.ClearTransferID(ctx, txID); err != nil {
				return nil, fmt.Errorf("unlink transaction: %w", err)
			}
			tx.TransferID = ""
		}
	}

	if err := s.transactions.UpdateType(ctx, txID, newType); err != nil {
		return nil, fmt.Errorf("update transaction type: %w", err)
	}
	tx.Type = newType

	return tx, nil
}

// LinkExisting links an already-imported transaction to a counterpart
// account by creating the missing leg automatically: same date, same
// currency, negated amount. This covers cases like recording a loan where
// the outgoing payment was already imported from a bank statement, but the
// counterpart (e.g. a borrower's external account) has no matching row yet.
// The existing transaction is also reclassified to "transfer", since linking
// it confirms that's what it actually is.
func (s *TransferService) LinkExisting(ctx context.Context, existingTxID, counterpartAccountID string) (*models.TransferResult, error) {
	if existingTxID == "" || counterpartAccountID == "" {
		return nil, fmt.Errorf("transaction_id and counterpart_account_id are required")
	}

	existingTx, err := s.transactions.GetByID(ctx, existingTxID)
	if err != nil {
		return nil, fmt.Errorf("lookup transaction: %w", err)
	}
	if existingTx == nil {
		return nil, ErrFromTxNotFound
	}
	if existingTx.TransferID != "" {
		return nil, ErrFromTxLinked
	}
	if existingTx.AccountID == counterpartAccountID {
		return nil, fmt.Errorf("counterpart_account_id must be different from the transaction's account")
	}

	counterpartAccount, err := s.accounts.FindByID(ctx, counterpartAccountID)
	if err != nil {
		return nil, fmt.Errorf("lookup counterpart account: %w", err)
	}
	if counterpartAccount == nil {
		return nil, fmt.Errorf("counterpart account not found")
	}

	counterpartTx, err := s.transactions.Create(ctx, &models.Transaction{
		AccountID:   counterpartAccountID,
		Date:        existingTx.Date,
		Description: existingTx.Description,
		Amount:      existingTx.Amount.Neg(),
		Type:        models.TypeTransfer,
		Currency:    existingTx.Currency,
	})
	if err != nil {
		return nil, fmt.Errorf("create counterpart transaction: %w", err)
	}

	if err := s.transactions.UpdateType(ctx, existingTxID, models.TypeTransfer); err != nil {
		return nil, fmt.Errorf("reclassify transaction: %w", err)
	}

	transfer, err := s.transfers.Create(ctx, existingTxID, counterpartTx.ID, nil, "manual")
	if err != nil {
		return nil, fmt.Errorf("link transfer: %w", err)
	}

	if err := s.transactions.SetTransferID(ctx, existingTxID, transfer.ID); err != nil {
		return nil, fmt.Errorf("link existing transaction: %w", err)
	}
	if err := s.transactions.SetTransferID(ctx, counterpartTx.ID, transfer.ID); err != nil {
		return nil, fmt.Errorf("link counterpart transaction: %w", err)
	}
	existingTx.TransferID = transfer.ID
	existingTx.Type = models.TypeTransfer
	counterpartTx.TransferID = transfer.ID

	return &models.TransferResult{
		Transfer:        *transfer,
		FromTransaction: *existingTx,
		ToTransaction:   *counterpartTx,
	}, nil
}

// fxRange bounds a plausible exchange rate for the weak cross-currency
// suggestion tier. Both rateAB (b amount / a amount) and rateBA (its inverse)
// are checked against it, so callers don't need to know which side of the
// pair is the "base" currency.
type fxRange struct {
	min, max float64
}

// MatchForPeriod fetches all accounts and their transactions in the given date
// range, runs the transfer matching algorithm, and returns the matched pairs.
// fxMin/fxMax optionally enable the fx_rate suggestion tier for cross-currency
// pairs (e.g. an informal currency exchange) whose implied rate falls in
// [fxMin, fxMax]; pass nil for either to disable it.
func (s *TransferService) MatchForPeriod(ctx context.Context, from, to time.Time, fxMin, fxMax *float64) ([]models.TransferMatch, error) {
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

	var fxr *fxRange
	if fxMin != nil && fxMax != nil {
		fxr = &fxRange{min: *fxMin, max: *fxMax}
	}
	pairs := s.Match(stmts, fxr)

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

	pairs := s.Match(stmts, nil)

	// Flatten in the same iteration order Match() used, so indices align.
	var flat []models.Transaction
	for _, stmt := range stmts {
		flat = append(flat, stmt.Transactions...)
	}

	linked := 0
	for _, pair := range pairs {
		// Skip the weakest tiers — they require user confirmation.
		if pair.confidence == models.MatchByAmountDate || pair.confidence == models.MatchByFXRate {
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

// Match identifies transfer pairs across statements using a priority order:
//  1. Same Reference across accounts (strongest)
//  2. Description contains counterpart's ShortNumber (TEF A/DE patterns)
//  3. Matching (or truncated-prefix) description, same currency
//  4. Same date + same absolute amount + same currency (weak, needs user confirmation)
//  5. Cross-currency pair whose implied rate falls in fxr, if provided (weak,
//     needs user confirmation) — e.g. an informal currency exchange
//
// Returns pairs of (a, b) indices into the flattened transaction list, with
// the confidence tier that produced the match.
func (s *TransferService) Match(statements []*models.Statement, fxr *fxRange) []matchPair {
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
			if confidence, ok := isTransferPair(a.tx, b.tx, shortNumbers[a.stmtIdx], shortNumbers[b.stmtIdx], fxr); ok {
				matched[i] = true
				matched[j] = true
				pairs = append(pairs, matchPair{a: i, b: j, confidence: confidence})
				break
			}
		}
	}

	return pairs
}

func isTransferPair(a, b models.Transaction, shortA, shortB string, fxr *fxRange) (models.MatchConfidence, bool) {
	// Cross-currency pairs can only match on implied exchange rate — none of
	// the same-currency tiers below are meaningful across currencies.
	if a.Currency != b.Currency {
		return matchFXRate(a, b, fxr)
	}

	// Tier 1: same non-empty reference
	if a.Reference != "" && a.Reference == b.Reference {
		return models.MatchByReference, true
	}

	// Tier 2: TEF description contains counterpart's short number AND amounts net to zero.
	// The description check alone is not sufficient — the same short number can appear
	// in unrelated transfers. Requiring amounts to cancel rules out false positives.
	if shortB != "" && strings.Contains(a.Description, shortB) &&
		a.Currency == b.Currency && a.Amount.Add(b.Amount).IsZero() {
		return models.MatchByShortNumber, true
	}
	if shortA != "" && strings.Contains(b.Description, shortA) &&
		a.Currency == b.Currency && a.Amount.Add(b.Amount).IsZero() {
		return models.MatchByShortNumber, true
	}

	// Tier 3: matching description + same currency + amounts net to zero.
	// Catches internal bank movements (e.g. BAC Objetivos) where both sides share
	// an identical (or prefix-truncated) description but carry no account-number
	// cross-reference. One side may be shorter due to PDF column-width truncation.
	if a.Currency == b.Currency && a.Amount.Add(b.Amount).IsZero() &&
		descriptionMatches(a.Description, b.Description) {
		return models.MatchByDescription, true
	}

	// Tier 4: same date + same currency + same absolute amount (opposite signs)
	if a.Date.Equal(b.Date) && a.Currency == b.Currency && a.Amount.Add(b.Amount).IsZero() {
		return models.MatchByAmountDate, true
	}

	return "", false
}

// matchFXRate matches two opposite-sign, opposite-currency transactions on
// the same date whose implied exchange rate falls within fxr. Checked in
// both directions (b/a and a/b) so callers don't need to know which side is
// the "base" currency for the range they supplied.
func matchFXRate(a, b models.Transaction, fxr *fxRange) (models.MatchConfidence, bool) {
	if fxr == nil {
		return "", false
	}
	if a.Amount.IsZero() || b.Amount.IsZero() || a.Amount.Sign() == b.Amount.Sign() {
		return "", false
	}
	if !a.Date.Equal(b.Date) {
		return "", false
	}

	aAbs, bAbs := a.Amount.Abs(), b.Amount.Abs()
	rateBA, _ := bAbs.Div(aAbs).Float64()
	rateAB, _ := aAbs.Div(bAbs).Float64()

	if (rateBA >= fxr.min && rateBA <= fxr.max) || (rateAB >= fxr.min && rateAB <= fxr.max) {
		return models.MatchByFXRate, true
	}
	return "", false
}

// descriptionMatches returns true when two descriptions are the same or one is
// a prefix of the other. The prefix check handles PDF truncation where a bank
// statement cuts a long description a few characters short (e.g.
// "BAC Objetivos Mante Carro" vs "BAC Objetivos Mante Car"). A minimum shared
// length of 8 characters prevents very short strings from producing false positives.
func descriptionMatches(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}
	if minLen < 8 {
		return false
	}
	return strings.HasPrefix(a, b) || strings.HasPrefix(b, a)
}
