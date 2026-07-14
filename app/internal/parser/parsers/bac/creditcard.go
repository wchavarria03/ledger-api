package bac

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"ledger-api/app/internal/models"
	"ledger-api/app/internal/parser"
)

func init() {
	parser.Register(&creditCardParser{})
}

type creditCardParser struct{}

func (p *creditCardParser) Name() string { return "bac/creditcard" }

// Detect identifies BAC credit card statements — a different layout from the
// checking/savings statements: dual-currency (Local/Dólares) columns, no
// running balance per row, headed by "Transacciones del periodo".
func (p *creditCardParser) Detect(text string) bool {
	return strings.Contains(text, "Transacciones del periodo") &&
		strings.Contains(text, "Producto:") &&
		strings.Contains(text, "Fecha de corte:")
}

var (
	ccProductPattern     = regexp.MustCompile(`Producto:\s*(\d{4}-\d{2}\*\*-\*{4}-\d{4})`)
	ccClosingDatePattern = regexp.MustCompile(`Fecha de corte:\s*(\d{2}/\d{2}/\d{4})`)
	// The interest charged this period isn't itemized as its own row in the
	// transactions table — it only shows up in this footer summary — so it's
	// pulled out separately and synthesized as its own expense transaction
	// below, dated at the statement's closing date.
	ccInterestLocalPattern   = regexp.MustCompile(`MES/LOCAL:\s*(-?[\d,]+\.\d{2})`)
	ccInterestDollarsPattern = regexp.MustCompile(`MES/D[OÓ]LARES:\s*(-?[\d,]+\.\d{2})`)
	// Applied globally (FindAllStringSubmatch) across the whole cell-delimited
	// statement text (see pdf.ExtractCellsFromBytes — this format's PDF has no
	// per-line structure to split on, so rows aren't isolated by newlines).
	// The description is non-greedy so it stops at the first point a valid
	// amount pair follows — safe now that cell boundaries are real whitespace,
	// unlike the raw PDF extraction where adjacent cells (e.g. a reference
	// number immediately followed by an amount) have no separator at all and
	// can't be reliably told apart.
	ccRowPattern = regexp.MustCompile(`\s*(?:(\d{2}/\d{2}/\d{4})\s+)?(.+?)\s+(-?[\d,]+\.\d{2})\s+(-?[\d,]+\.\d{2})`)
)

// ccRow is one parsed row before it's split into per-currency transactions.
type ccRow struct {
	date        time.Time
	description string
	local       decimal.Decimal
	dollars     decimal.Decimal
}

func (p *creditCardParser) Parse(text string) (*models.Statement, error) {
	productMatch := ccProductPattern.FindStringSubmatch(text)
	if productMatch == nil {
		return nil, fmt.Errorf("bac/creditcard: could not find product number")
	}
	productNumber := productMatch[1]

	var closingDate time.Time
	if m := ccClosingDatePattern.FindStringSubmatch(text); m != nil {
		if t, err := time.Parse("02/01/2006", m[1]); err == nil {
			closingDate = t
		}
	}

	var prevBalanceLocal, prevBalanceDollars decimal.Decimal
	havePrevBalance := false
	var rows []ccRow

	for _, m := range ccRowPattern.FindAllStringSubmatch(text, -1) {
		dateStr := m[1]
		description := strings.TrimSpace(m[2])
		local := parseAmount(m[3])
		dollars := parseAmount(m[4])

		if strings.Contains(description, "Previous balance") {
			prevBalanceLocal, prevBalanceDollars = local, dollars
			havePrevBalance = true
			continue
		}

		if local.IsZero() && dollars.IsZero() {
			continue // informational match (headers, rates, cash-back, limits...)
		}

		// Undated-but-monetary rows (e.g. "REVERSION INTERES CORRIENTES
		// PERIODO") apply to the whole billing period — stamp them with the
		// statement's closing date.
		date := closingDate
		if dateStr != "" {
			t, err := time.Parse("02/01/2006", dateStr)
			if err != nil {
				continue
			}
			date = t
		}

		rows = append(rows, ccRow{date: date, description: description, local: local, dollars: dollars})
	}

	if !havePrevBalance {
		return nil, fmt.Errorf("bac/creditcard: could not find previous balance")
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("bac/creditcard: no transactions found — verify the PDF matches this format")
	}

	// The PDF's own sign convention is inverted from the rest of this app: a
	// purchase (which increases what you owe) prints as positive, and a
	// payment/credit (which reduces it) prints as negative. Negating puts
	// purchases in the red like every other expense and payments/credits in
	// the green, consistent with checking/savings accounts. Balances are
	// tracked in the same inverted sign, so a running balance is a negative
	// "amount owed" — like a liability.
	runningLocal := prevBalanceLocal.Neg()
	runningDollars := prevBalanceDollars.Neg()

	var transactions []models.Transaction
	for _, r := range rows {
		if !r.local.IsZero() {
			amount := r.local.Neg()
			runningLocal = runningLocal.Add(amount)
			transactions = append(transactions, models.Transaction{
				Date:        r.date,
				Type:        deriveCreditCardType(r.description, amount),
				Description: r.description,
				Amount:      amount,
				Balance:     runningLocal,
				Currency:    "CRC",
			})
		}
		if !r.dollars.IsZero() {
			amount := r.dollars.Neg()
			runningDollars = runningDollars.Add(amount)
			transactions = append(transactions, models.Transaction{
				Date:        r.date,
				Type:        deriveCreditCardType(r.description, amount),
				Description: r.description,
				Amount:      amount,
				Balance:     runningDollars,
				Currency:    "USD",
			})
		}
	}

	if m := ccInterestLocalPattern.FindStringSubmatch(text); m != nil {
		if interest := parseAmount(m[1]); interest.IsPositive() {
			amount := interest.Neg()
			runningLocal = runningLocal.Add(amount)
			transactions = append(transactions, models.Transaction{
				Date:        closingDate,
				Type:        models.TypeExpense,
				Description: "Intereses corrientes del mes",
				Amount:      amount,
				Balance:     runningLocal,
				Currency:    "CRC",
			})
		}
	}
	if m := ccInterestDollarsPattern.FindStringSubmatch(text); m != nil {
		if interest := parseAmount(m[1]); interest.IsPositive() {
			amount := interest.Neg()
			runningDollars = runningDollars.Add(amount)
			transactions = append(transactions, models.Transaction{
				Date:        closingDate,
				Type:        models.TypeExpense,
				Description: "Intereses corrientes del mes",
				Amount:      amount,
				Balance:     runningDollars,
				Currency:    "USD",
			})
		}
	}

	stampImportSeq(transactions)

	return &models.Statement{
		AccountNumber: productNumber,
		Transactions:  transactions,
	}, nil
}

// deriveCreditCardType classifies a credit-card statement row. amount is
// already sign-inverted to match the app's convention (purchases negative).
//
// "SU PAGO RECIBIDO" rows are the credit-card side of a payment already
// recorded as an outgoing transaction on a checking/savings account, so
// they're a transfer, not income. Any other credit (e.g. an interest
// reversal) is treated as a refund (income); ordinary purchases are expenses.
func deriveCreditCardType(description string, amount decimal.Decimal) models.TransactionType {
	if strings.Contains(strings.ToUpper(description), "SU PAGO RECIBIDO") {
		return models.TypeTransfer
	}
	if amount.IsPositive() {
		return models.TypeIncome
	}
	return models.TypeExpense
}
