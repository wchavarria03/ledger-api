package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"ledger-api/app/internal/models"
	"ledger-api/app/internal/parser"
	_ "ledger-api/app/internal/parser/parsers/bac"
	"ledger-api/app/internal/pdf"
)

const maxUploadBytes = 10 << 20 // 10 MB

func NewUploadHandler(importer StatementImporter) *UploadHandler {
	return &UploadHandler{importer: importer}
}

// Import handles POST /v1/import.
// With ?dry_run=true it parses the PDF and returns a preview without storing anything.
// Without dry_run it imports and returns an ImportSummary.
func (h *UploadHandler) Import(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadBytes)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file field is required"})
		return
	}
	defer file.Close()

	if !strings.HasSuffix(strings.ToLower(header.Filename), ".pdf") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "only PDF files are supported"})
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read uploaded file"})
		return
	}

	text, err := pdf.ExtractTextFromBytes(data)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "could not extract text from PDF"})
		return
	}

	p, err := parser.Detect(text)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "unsupported bank statement format — only BAC statements are supported right now"})
		return
	}

	stmt, err := p.Parse(text)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "failed to parse statement: " + err.Error()})
		return
	}
	stmt.SourceFile = header.Filename

	if c.Query("dry_run") == "true" {
		preview := buildPreview(stmt, p.Name())
		if count, err := h.importer.CheckOverlap(c.Request.Context(), stmt); err == nil {
			preview.ExistingCount = count
		}
		c.JSON(http.StatusOK, preview)
		return
	}

	if raw := c.PostForm("overrides"); raw != "" {
		var overrides []models.TransactionOverride
		if err := json.Unmarshal([]byte(raw), &overrides); err == nil {
			applyOverrides(stmt, overrides)
		}
	}

	summary, err := h.importer.ImportWithSummary(c.Request.Context(), stmt, p.Name())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "import failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, summary)
}

func applyOverrides(stmt *models.Statement, overrides []models.TransactionOverride) {
	for _, o := range overrides {
		if o.Index < 0 || o.Index >= len(stmt.Transactions) {
			continue
		}
		tx := &stmt.Transactions[o.Index]
		if o.Date != "" {
			if t, err := time.Parse("2006-01-02", o.Date); err == nil {
				tx.Date = t
			}
		}
		if o.Description != "" {
			tx.Description = o.Description
		}
		switch models.TransactionType(o.Type) {
		case models.TypeExpense, models.TypeIncome, models.TypeTransfer:
			tx.Type = models.TransactionType(o.Type)
		}
	}
}

func buildPreview(stmt *models.Statement, parserName string) *models.ImportPreview {
	bank := parserName
	if idx := strings.Index(parserName, "/"); idx != -1 {
		bank = parserName[:idx]
	}

	currency := "CRC"
	if len(stmt.Transactions) > 0 {
		currency = stmt.Transactions[0].Currency
	}

	var periodStart, periodEnd string
	if len(stmt.Transactions) > 0 {
		periodStart = stmt.Transactions[0].Date.Format("2006-01-02")
		periodEnd = stmt.Transactions[len(stmt.Transactions)-1].Date.Format("2006-01-02")
	}

	txs := make([]models.Transaction, len(stmt.Transactions))
	copy(txs, stmt.Transactions)

	return &models.ImportPreview{
		AccountNumber:    stmt.AccountNumber,
		Bank:             strings.ToUpper(bank),
		Currency:         currency,
		TransactionCount: len(stmt.Transactions),
		PeriodStart:      periodStart,
		PeriodEnd:        periodEnd,
		Transactions:     txs,
	}
}
