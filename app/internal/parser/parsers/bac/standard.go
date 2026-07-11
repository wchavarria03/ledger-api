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

var ibanPattern = regexp.MustCompile(`^CR\d{20}$`)

const (
	bacFieldCount  = 7  // columns per transaction row: Fecha|Referencia|Código|Descripción|Débito|Crédito|Balance
	bacShortStart  = 10 // start index of 9-digit short number within the 20 IBAN digits
	bacShortEnd    = 19 // end index (exclusive)
)

func init() {
	parser.Register(&standardParser{})
}

type standardParser struct{}

func (p *standardParser) Name() string { return "bac/standard" }

// Detect identifies BAC statements by the presence of "Balance" as a standalone
// line combined with "Resumen de", which are markers unique to this format.
func (p *standardParser) Detect(text string) bool {
	hasBalance := false
	hasResumen := false
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "Balance" {
			hasBalance = true
		}
		if strings.HasPrefix(trimmed, "Resumen de") {
			hasResumen = true
		}
		if hasBalance && hasResumen {
			return true
		}
	}
	return false
}

func (p *standardParser) Parse(text string) (*models.Statement, error) {
	lines := strings.Split(text, "\n")

	accountNumber := ""
	currency := "CRC"
	nextIsCurrency := false
	inTable := false
	fields := make([]string, 0, 7)
	var transactions []models.Transaction

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Extract account number from IBAN line (CR + 20 digits).
		// Only capture the first occurrence (page 1 header).
		if accountNumber == "" && ibanPattern.MatchString(trimmed) {
			accountNumber = trimmed
			continue
		}

		// "Moneda" column header is immediately followed by the currency value (CRC/USD).
		if trimmed == "Moneda" {
			nextIsCurrency = true
			continue
		}
		if nextIsCurrency {
			currency = trimmed
			nextIsCurrency = false
			continue
		}

		if trimmed == "" {
			continue
		}

		// "Resumen de" marks the end of all transaction data.
		if strings.HasPrefix(trimmed, "Resumen de") {
			break
		}

		// Standalone "Balance" is the column header — enter (or re-enter) table mode.
		// On page 2 this also discards any partial fields accumulated from page headers.
		if trimmed == "Balance" {
			inTable = true
			fields = fields[:0]
			continue
		}

		if !inTable {
			continue
		}

		// Skip known page-header lines that repeat on every page. Without this,
		// content like "Saldo en libros*" or "Fecha desde:" that appears in the
		// page-2 header while inTable=true would pollute the field accumulator
		// and could produce a spurious transaction if a valid date happens to
		// land at position [0] of a 7-field group.
		if isPageHeaderLine(trimmed) {
			continue
		}

		fields = append(fields, trimmed)

		if len(fields) == bacFieldCount {
			tx, err := parseFields(fields, currency)
			if err == nil {
				transactions = append(transactions, tx)
			}
			fields = fields[:0]
		}
	}

	if len(transactions) == 0 {
		return nil, fmt.Errorf("bac/standard: no transactions found — verify the PDF matches this format")
	}

	return &models.Statement{
		AccountNumber: accountNumber,
		ShortNumber:   bacShortNumber(accountNumber),
		Transactions:  transactions,
	}, nil
}

// bacShortNumber extracts the 9-digit short account number BAC uses inside
// transfer descriptions (e.g. "TEF A : 933175556").
// BAC IBANs are CR + 20 digits; the short number is digits [10:19].
func bacShortNumber(iban string) string {
	if !ibanPattern.MatchString(iban) {
		return ""
	}
	digits := iban[2:]
	if len(digits) < 19 {
		return ""
	}
	return digits[bacShortStart:bacShortEnd]
}

// parseFields converts the 7 raw text fields into a Transaction.
// BAC statement column order: Fecha | Referencia | Código | Descripción | Débito | Crédito | Balance
func parseFields(fields []string, currency string) (models.Transaction, error) {
	date, err := parseDate(fields[0])
	if err != nil {
		return models.Transaction{}, fmt.Errorf("invalid date %q: %w", fields[0], err)
	}

	if !isNumeric(fields[4]) || !isNumeric(fields[5]) {
		return models.Transaction{}, fmt.Errorf("non-numeric amounts: debit=%q credit=%q", fields[4], fields[5])
	}

	debit := parseAmount(fields[4])
	credit := parseAmount(fields[5])

	amount := credit
	if debit.IsPositive() {
		amount = debit.Neg()
	}

	return models.Transaction{
		Date:        date,
		Reference:   fields[1],
		Code:        fields[2],
		Type:        deriveType(fields[2], fields[3], amount),
		Description: fields[3],
		Amount:      amount,
		Balance:     parseAmount(fields[6]),
		Currency:    currency,
	}, nil
}

func deriveType(code, description string, amount decimal.Decimal) models.TransactionType {
	desc := strings.ToUpper(description)

	if strings.Contains(desc, "COMISION") || strings.Contains(desc, "COBRO ADMINISTR") {
		return models.TypeExpense
	}
	// Interest can go either way: a debit (interest charged on a loan/overdraft)
	// is an expense, but a credit (interest the bank pays you, e.g. on a
	// savings/investment account) is income. Only force expense when the
	// amount is actually negative — otherwise fall through to the default
	// sign-based classification below.
	if strings.Contains(desc, "INTERES") && amount.IsNegative() {
		return models.TypeExpense
	}
	// BAC also codes some third-party SINPE Móvil payments (e.g. utility bills)
	// as "TF", not just real inter-account transfers. Require the description to
	// carry transfer-like language too, matching the signal deriveSavingsType
	// uses ("TEF A"/"TEF DE" for outbound/inbound electronic transfers).
	if code == "TF" && strings.HasPrefix(desc, "TEF") {
		return models.TypeTransfer
	}
	if amount.IsNegative() {
		return models.TypeExpense
	}
	return models.TypeIncome
}

func parseDate(s string) (time.Time, error) {
	formats := []string{"02/01/2006", "01/02/2006", "2006-01-02", "02-01-2006"}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized date format")
}

func parseAmount(s string) decimal.Decimal {
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, " ", "")
	d, _ := decimal.NewFromString(s)
	return d
}

func isNumeric(s string) bool {
	cleaned := strings.ReplaceAll(s, ",", "")
	_, err := decimal.NewFromString(cleaned)
	return err == nil
}

// isPageHeaderLine returns true for lines that are part of the BAC page header
// repeated on every page. These must not enter the field accumulator while
// inTable=true or they can form spurious transactions.
func isPageHeaderLine(s string) bool {
	switch s {
	case "Transacciones del mes",
		"Cuenta:",
		"Moneda:",
		"Balance de la cuenta",
		"Saldo en libros*",
		"Retenidos y diferidos",
		"Saldo disponible",
		"Débito", "Debito",
		"Créditos", "Creditos",
		"Fecha",
		"Referencia Codigó Descripción":
		return true
	}
	return strings.HasPrefix(s, "Fecha de generación:") ||
		strings.HasPrefix(s, "Fecha desde:") ||
		strings.HasPrefix(s, "Fecha hasta:") ||
		strings.HasPrefix(s, "* El Saldo en libros") ||
		strings.HasPrefix(s, "pendientes de confirmación") ||
		strings.HasPrefix(s, "Este documento es de referencia")
}
