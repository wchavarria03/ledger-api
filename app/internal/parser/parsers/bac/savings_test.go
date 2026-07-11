package bac_test

import (
	"fmt"
	"testing"

	"ledger-api/app/internal/parser"
	_ "ledger-api/app/internal/parser/parsers/bac"
)

// sampleSavingsText mirrors the one-cell-per-line output produced by ledongthuc/pdf.
const sampleSavingsText = `
Nombre:
WALTER CHAVARRIA MORA
Cuenta IBAN:
CR34 0102 0000 9361 1222 62
Moneda:
U.S. DOLLAR
Número(s) SINPE
Móvil Asociado(s):
No tiene celulares asociados. Si desea
afiliarse ingrese a Banca en Línea.
3101012009
30/JUN/26
Tasas de interés escalonadas
CUADRO RESUMEN
DÉBITOS
CRÉDITOS
SALDOS
TOTAL
MONTO
TOTAL
MONTO
SALDO PROMEDIO
SALDO ANTERIOR
SALDO A LA FECHA
Cuenta no paga intereses
1
1,400.00
6
380.00
2,644.95
3,624.35
2,604.35
Estado de Cuenta:
 ACTIVA
Marca:
SERVICIO AL CLIENTE
2295-9797
NO. REFERENCIA
FECHA
CONCEPTO
DÉBITOS
CRÉDITOS
000019643
JUN/01
BAC Objetivos Mante Carro
60.00
000055425
JUN/02
BAC Objetivos Marchamo
50.00
000055426
JUN/02
BAC Objetivos Viajes
80.00
406452356
JUN/04
TEF A : 701979726
1,400.00
000083485
JUN/16
BAC Objetivos Marchamo
50.00
000083486
JUN/16
BAC Objetivos Viajes
80.00
000083487
JUN/16
BAC Objetivos Mante Carro
60.00
ÚLTIMA LÍNEA
SALDO AL CORTE
2,604.35
https://bac.cr/EC-Seguridad-E26 link sospechoso
`

func TestSavingsParser(t *testing.T) {
	p, err := parser.Detect(sampleSavingsText)
	if err != nil {
		t.Fatal("Detect:", err)
	}
	if p.Name() != "bac/savings" {
		t.Fatalf("expected bac/savings, got %s", p.Name())
	}

	stmt, err := p.Parse(sampleSavingsText)
	if err != nil {
		t.Fatal("Parse:", err)
	}
	fmt.Printf("AccountNumber: %s\nShortNumber:   %s\nTransactions:  %d\n\n",
		stmt.AccountNumber, stmt.ShortNumber, len(stmt.Transactions))
	for _, tx := range stmt.Transactions {
		fmt.Printf("  %s  ref=%-12s  %-35s  amt=%9s  bal=%s  type=%s\n",
			tx.Date.Format("2006-01-02"),
			tx.Reference,
			tx.Description,
			tx.Amount.StringFixed(2),
			tx.Balance.StringFixed(2),
			tx.Type,
		)
	}

	if len(stmt.Transactions) != 7 {
		t.Errorf("expected 7 transactions, got %d", len(stmt.Transactions))
	}
	if stmt.AccountNumber != "CR34010200009361122262" {
		t.Errorf("unexpected account number: %s", stmt.AccountNumber)
	}
	if stmt.ShortNumber != "936112226" {
		t.Errorf("unexpected short number: %s", stmt.ShortNumber)
	}
	// TEF A must be negative (debit)
	tefTx := stmt.Transactions[3]
	if !tefTx.Amount.IsNegative() {
		t.Errorf("TEF A should be negative, got %s", tefTx.Amount)
	}
	if tefTx.Amount.StringFixed(2) != "-1400.00" {
		t.Errorf("TEF A amount: expected -1400.00, got %s", tefTx.Amount.StringFixed(2))
	}
	// BAC Objetivos must be positive (credit)
	if stmt.Transactions[0].Amount.IsNegative() {
		t.Errorf("BAC Objetivos should be positive, got %s", stmt.Transactions[0].Amount)
	}
	// Last balance must equal closing balance
	if stmt.Transactions[6].Balance.StringFixed(2) != "2604.35" {
		t.Errorf("last tx balance: expected 2604.35, got %s", stmt.Transactions[6].Balance.StringFixed(2))
	}
	// Reconstructed opening balance must equal SALDO ANTERIOR
	opening := stmt.Transactions[0].Balance.Sub(stmt.Transactions[0].Amount)
	if opening.StringFixed(2) != "3624.35" {
		t.Errorf("reconstructed opening balance: expected 3624.35, got %s", opening.StringFixed(2))
	}
}
