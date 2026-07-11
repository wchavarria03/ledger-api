package bac

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"ledger-api/app/internal/models"
	"ledger-api/app/internal/parser"
)

// savingsMonths maps Spanish 3-letter month abbreviations used in BAC savings statements.
var savingsMonths = map[string]int{
	"ENE": 1, "FEB": 2, "MAR": 3, "ABR": 4, "MAY": 5, "JUN": 6,
	"JUL": 7, "AGO": 8, "SEP": 9, "OCT": 10, "NOV": 11, "DIC": 12,
}

// savingsCutoffRe matches the cut-off date embedded in header lines like "3101012009 30/JUN/26".
var savingsCutoffRe = regexp.MustCompile(`(\d{2})/([A-Z]{3})/(\d{2,4})`)

// savingsTxRe matches a single-line transaction row from the PDF extraction:
// [reference digits]  [MMM/DD date]  [description words]  [amount]
// Non-greedy description capture ensures the trailing amount is always the last token.
var savingsTxRe = regexp.MustCompile(`^(\d+)\s+([A-Z]{3}/\d{2})\s+(.+?)\s+([\d,]+\.\d{2})$`)

func init() {
	parser.Register(&savingsParser{})
}

type savingsParser struct{}

func (p *savingsParser) Name() string { return "bac/savings" }

// Detect identifies BAC savings statements by markers unique to this format.
// Standard checking statements use "Balance" + "Resumen de"; savings use
// "SALDO ANTERIOR" + "ÚLTIMA LÍNEA".
func (p *savingsParser) Detect(text string) bool {
	return strings.Contains(text, "SALDO ANTERIOR") && strings.Contains(text, "ÚLTIMA LÍNEA")
}

func (p *savingsParser) Parse(text string) (*models.Statement, error) {
	lines := strings.Split(text, "\n")

	var (
		accountNumber      string
		currency           = "CRC"
		cutoffYear         int
		cutoffMonth        int
		inTable            bool
		closingBalance     decimal.Decimal
		closingBalanceSet  bool
		transactions       []models.Transaction
	)

	for _, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" {
			continue
		}

		// IBAN: "Cuenta IBAN: CR34 0102 0000 9361 1222 62"
		// Strip label and spaces, then validate against the shared ibanPattern.
		if accountNumber == "" && strings.HasPrefix(trimmed, "Cuenta IBAN:") {
			raw := strings.TrimPrefix(trimmed, "Cuenta IBAN:")
			raw = strings.ReplaceAll(strings.TrimSpace(raw), " ", "")
			if ibanPattern.MatchString(raw) {
				accountNumber = raw
			}
			continue
		}

		// Currency: "Moneda: U.S. DOLLAR" → "USD"; colones is the default ("CRC").
		if strings.HasPrefix(trimmed, "Moneda:") {
			moneda := strings.ToUpper(strings.TrimSpace(strings.TrimPrefix(trimmed, "Moneda:")))
			if strings.Contains(moneda, "DOLLAR") || strings.Contains(moneda, "USD") {
				currency = "USD"
			} else if strings.Contains(moneda, "EURO") {
				currency = "EUR"
			}
			continue
		}

		// Cut-off date from header lines like "3101012009 30/JUN/26".
		// We need the year (and month) for per-transaction date reconstruction.
		if cutoffYear == 0 && !inTable {
			if m := savingsCutoffRe.FindStringSubmatch(trimmed); m != nil {
				if mo, ok := savingsMonths[m[2]]; ok {
					cutoffMonth = mo
					if y, err := savingsParseYear(m[3]); err == nil {
						cutoffYear = y
					}
				}
			}
		}

		// "NO. REFERENCIA FECHA CONCEPTO DÉBITOS CRÉDITOS" marks the start of transactions.
		if strings.HasPrefix(trimmed, "NO. REFERENCIA") {
			inTable = true
			continue
		}

		if !inTable {
			continue
		}

		// "ÚLTIMA LÍNEA SALDO AL CORTE 2,604.35" ends the table.
		if strings.HasPrefix(trimmed, "ÚLTIMA LÍNEA") {
			parts := strings.Fields(trimmed)
			if len(parts) > 0 {
				closingBalance = parseAmount(parts[len(parts)-1])
				closingBalanceSet = true
			}
			break
		}

		tx, err := parseSavingsLine(trimmed, currency, cutoffYear, cutoffMonth)
		if err != nil {
			continue // non-transaction lines between header and ÚLTIMA LÍNEA are skipped
		}
		transactions = append(transactions, tx)
	}

	if len(transactions) == 0 {
		return nil, fmt.Errorf("bac/savings: no transactions found — verify the PDF matches this format")
	}

	// Reconstruct the per-transaction running balance by walking backwards from the closing balance.
	// Each transaction's balance is the account balance AFTER that transaction applied.
	if closingBalanceSet {
		running := closingBalance
		for i := len(transactions) - 1; i >= 0; i-- {
			transactions[i].Balance = running
			running = running.Sub(transactions[i].Amount)
		}
	}

	return &models.Statement{
		AccountNumber: accountNumber,
		ShortNumber:   bacShortNumber(accountNumber),
		Transactions:  transactions,
	}, nil
}

func parseSavingsLine(line, currency string, cutoffYear, cutoffMonth int) (models.Transaction, error) {
	m := savingsTxRe.FindStringSubmatch(line)
	if m == nil {
		return models.Transaction{}, fmt.Errorf("does not match transaction pattern")
	}
	// m[1]=reference  m[2]=date(MMM/DD)  m[3]=description  m[4]=amount

	date, err := parseSavingsDate(m[2], cutoffYear, cutoffMonth)
	if err != nil {
		return models.Transaction{}, err
	}

	absAmount := parseAmount(m[4])
	description := m[3]

	amount := absAmount
	if isSavingsDebit(description) {
		amount = absAmount.Neg()
	}

	return models.Transaction{
		Date:        date,
		Reference:   m[1],
		Type:        deriveSavingsType(description, amount),
		Description: description,
		Amount:      amount,
		Currency:    currency,
	}, nil
}

// parseSavingsDate converts "JUN/01" to a time.Time using the statement cut-off year.
// If the transaction month exceeds the cut-off month, the transaction is from the prior year
// (handles statements that span a year boundary, e.g. cut-off JAN/26 with DEC transactions).
func parseSavingsDate(s string, cutoffYear, cutoffMonth int) (time.Time, error) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("invalid savings date %q", s)
	}
	month, ok := savingsMonths[parts[0]]
	if !ok {
		return time.Time{}, fmt.Errorf("unknown month abbreviation %q", parts[0])
	}
	day, err := strconv.Atoi(parts[1])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid day %q in savings date", parts[1])
	}
	year := cutoffYear
	if month > cutoffMonth {
		year--
	}
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC), nil
}

func savingsParseYear(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	if n < 100 {
		n += 2000
	}
	return n, nil
}

// isSavingsDebit returns true when the description signals money leaving this account.
// BAC savings accounts use "TEF A" for outbound transfers; purchases and fees are
// also debits. Everything else (BAC Objetivos releases, TEF DE, interest) is a credit.
func isSavingsDebit(description string) bool {
	desc := strings.ToUpper(description)
	return strings.HasPrefix(desc, "TEF A") ||
		strings.Contains(desc, "PAGO ") ||
		strings.Contains(desc, "COMPRA") ||
		strings.Contains(desc, "COMISION") ||
		strings.Contains(desc, "COBRO") ||
		strings.Contains(desc, "RETIRO") ||
		strings.Contains(desc, "CARGO") ||
		strings.Contains(desc, "DEBITO AUTO")
}

// deriveSavingsType classifies the transaction type for a savings account row.
func deriveSavingsType(description string, amount decimal.Decimal) models.TransactionType {
	desc := strings.ToUpper(description)

	if strings.Contains(desc, "COMISION") || strings.Contains(desc, "COBRO ADMINISTR") {
		return models.TypeExpense
	}
	if strings.Contains(desc, "INTERES") && amount.IsNegative() {
		return models.TypeExpense
	}
	// TEF (electronic transfer): both outbound (TEF A) and inbound (TEF DE)
	if strings.HasPrefix(desc, "TEF ") {
		return models.TypeTransfer
	}
	// BAC savings goals releasing funds back to the main account
	if strings.HasPrefix(desc, "BAC OBJETIVOS") {
		return models.TypeTransfer
	}
	if amount.IsNegative() {
		return models.TypeExpense
	}
	return models.TypeIncome
}
