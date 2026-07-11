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

// savingsCutoffRe matches date lines like "30/JUN/26" that carry the statement cut-off date.
var savingsCutoffRe = regexp.MustCompile(`^(\d{2})/([A-Z]{3})/(\d{2,4})$`)

const savingsFieldCount = 4 // ref | date | description | amount (one column — debit or credit)

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
		accountNumber     string
		currency          = "CRC"
		cutoffYear        int
		cutoffMonth       int
		inTable           bool
		afterUltimaLinea  int // 0 = not seen; 1 = "ÚLTIMA LÍNEA" seen; 2 = "SALDO AL CORTE" seen
		closingBalance    decimal.Decimal
		closingBalanceSet bool
		transactions      []models.Transaction
		fields            []string
		nextIsIBAN        bool
		nextIsCurrency    bool
	)

	for _, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" {
			continue
		}

		// After ÚLTIMA LÍNEA: consume "SALDO AL CORTE" then the balance value.
		if afterUltimaLinea > 0 {
			if afterUltimaLinea == 1 {
				// Expect "SALDO AL CORTE" next
				afterUltimaLinea = 2
				continue
			}
			// afterUltimaLinea == 2: this line is the closing balance
			closingBalance = parseAmount(trimmed)
			closingBalanceSet = true
			break
		}

		// IBAN: "Cuenta IBAN:" appears on its own line; the value is the next line.
		if nextIsIBAN {
			raw := strings.ReplaceAll(trimmed, " ", "")
			if ibanPattern.MatchString(raw) {
				accountNumber = raw
			}
			nextIsIBAN = false
			continue
		}
		if trimmed == "Cuenta IBAN:" {
			nextIsIBAN = true
			continue
		}

		// Currency: "Moneda:" on its own line; value is the next line.
		if nextIsCurrency {
			moneda := strings.ToUpper(trimmed)
			if strings.Contains(moneda, "DOLLAR") || strings.Contains(moneda, "USD") {
				currency = "USD"
			} else if strings.Contains(moneda, "EURO") {
				currency = "EUR"
			}
			nextIsCurrency = false
			continue
		}
		if trimmed == "Moneda:" {
			nextIsCurrency = true
			continue
		}

		// Cut-off date from a standalone line like "30/JUN/26".
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

		// "NO. REFERENCIA" is the column-header line that starts the transaction table.
		if trimmed == "NO. REFERENCIA" {
			inTable = true
			fields = fields[:0]
			continue
		}

		if !inTable {
			continue
		}

		// "ÚLTIMA LÍNEA" ends the transaction table.
		if trimmed == "ÚLTIMA LÍNEA" {
			afterUltimaLinea = 1
			continue
		}

		// Skip the remaining column-header lines that appear immediately after
		// "NO. REFERENCIA" and would pollute the field accumulator.
		if isSavingsColumnHeader(trimmed) {
			continue
		}

		fields = append(fields, trimmed)

		if len(fields) == savingsFieldCount {
			tx, err := parseSavingsFields(fields, currency, cutoffYear, cutoffMonth)
			if err == nil {
				transactions = append(transactions, tx)
			}
			fields = fields[:0]
		}
	}

	if len(transactions) == 0 {
		return nil, fmt.Errorf("bac/savings: no transactions found — verify the PDF matches this format")
	}

	// Reconstruct the per-transaction running balance by walking backwards from the closing
	// balance. Each stored balance is the account value AFTER that transaction applied.
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

// isSavingsColumnHeader returns true for the column-header lines that appear once
// after "NO. REFERENCIA" and must not enter the field accumulator.
func isSavingsColumnHeader(s string) bool {
	switch s {
	case "FECHA", "CONCEPTO", "DÉBITOS", "CRÉDITOS":
		return true
	}
	return false
}

// parseSavingsFields converts 4 raw lines into a Transaction.
// Column order: reference | date (MMM/DD) | description | amount (always positive in PDF)
func parseSavingsFields(fields []string, currency string, cutoffYear, cutoffMonth int) (models.Transaction, error) {
	date, err := parseSavingsDate(fields[1], cutoffYear, cutoffMonth)
	if err != nil {
		return models.Transaction{}, err
	}

	absAmount := parseAmount(fields[3])
	if absAmount.IsZero() {
		return models.Transaction{}, fmt.Errorf("zero amount for ref %s", fields[0])
	}
	description := fields[2]

	amount := absAmount
	if isSavingsDebit(description) {
		amount = absAmount.Neg()
	}

	return models.Transaction{
		Date:        date,
		Reference:   fields[0],
		Type:        deriveSavingsType(description, amount),
		Description: description,
		Amount:      amount,
		Currency:    currency,
	}, nil
}

// parseSavingsDate converts "JUN/01" to time.Time using the statement cut-off year.
// If the transaction month exceeds the cut-off month, the transaction belongs to the prior
// year (handles statements that span a year boundary, e.g. cut-off JAN/26 with DEC entries).
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
// BAC savings uses "TEF A" for outbound electronic transfers. Everything else
// (BAC Objetivos goal releases, "TEF DE" inbound, interest) is a credit.
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
	if strings.HasPrefix(desc, "TEF ") {
		return models.TypeTransfer
	}
	if strings.HasPrefix(desc, "BAC OBJETIVOS") {
		return models.TypeTransfer
	}
	if amount.IsNegative() {
		return models.TypeExpense
	}
	return models.TypeIncome
}
