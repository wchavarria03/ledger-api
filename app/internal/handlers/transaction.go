package handlers

import (
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"ledger-api/app/internal/models"
)

func NewTransactionHandler(svc TransactionLister) *TransactionHandler {
	return &TransactionHandler{svc: svc}
}

func (h *TransactionHandler) Create(c *gin.Context) {
	var req createTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	txType := models.TransactionType(req.Type)
	switch txType {
	case models.TypeExpense, models.TypeIncome, models.TypeTransfer:
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "type must be expense, income, or transfer"})
		return
	}

	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date, expected YYYY-MM-DD"})
		return
	}

	tx, err := h.svc.Create(c.Request.Context(), &models.Transaction{
		AccountID:   c.Param("id"),
		Date:        date,
		Description: req.Description,
		Amount:      decimal.NewFromFloat(req.Amount),
		Type:        txType,
		Currency:    req.Currency,
		Reference:   req.Reference,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create transaction"})
		return
	}

	c.JSON(http.StatusCreated, tx)
}

func (h *TransactionHandler) ListByAccount(c *gin.Context) {
	accountID := c.Param("id")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	filter := models.TxFilter{
		Search: c.Query("search"),
		Type:   c.Query("type"),
		From:   c.Query("from"),
		To:     c.Query("to"),
		Page:   page,
		Limit:  limit,
	}

	txs, total, err := h.svc.ListFiltered(c.Request.Context(), accountID, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	c.JSON(http.StatusOK, models.TxPage{
		Transactions: txs,
		Total:        total,
		Page:         page,
		Limit:        limit,
		TotalPages:   totalPages,
	})
}
