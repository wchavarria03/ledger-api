package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"ledger-api/app/internal/models"
	"ledger-api/app/internal/services"
)

func NewTransferHandler(svc TransferService) *TransferHandler {
	return &TransferHandler{svc: svc}
}

// Create handles POST /v1/transfers — records money moving from one
// account to another as a linked pair of transactions.
func (h *TransferHandler) Create(c *gin.Context) {
	var req createTransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date, expected YYYY-MM-DD"})
		return
	}

	result, err := h.svc.CreateTransfer(c.Request.Context(), models.TransferInput{
		FromAccountID: req.FromAccountID,
		ToAccountID:   req.ToAccountID,
		Amount:        decimal.NewFromFloat(req.Amount),
		Currency:      req.Currency,
		Date:          date,
		Description:   req.Description,
	})
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, result)
}

// Link handles POST /v1/transfers/link — links two already-imported transactions
// as a matched transfer pair without creating any new transaction rows.
func (h *TransferHandler) Link(c *gin.Context) {
	var req linkTransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.FromTxID == req.ToTxID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "from_tx_id and to_tx_id must be different"})
		return
	}

	result, err := h.svc.LinkTransactions(c.Request.Context(), req.FromTxID, req.ToTxID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrFromTxNotFound), errors.Is(err, services.ErrToTxNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, services.ErrFromTxLinked), errors.Is(err, services.ErrToTxLinked):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, result)
}

// UpdateTransactionType handles PATCH /v1/transactions/:id/type — corrects a
// transaction's type after import, e.g. when the parser miscoded a payment
// to an external party as a transfer. If the transaction was linked to a
// transfer, the link is torn down as part of the update.
func (h *TransferHandler) UpdateTransactionType(c *gin.Context) {
	txID := c.Param("id")
	var req updateTransactionTypeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tx, err := h.svc.UpdateTransactionType(c.Request.Context(), txID, models.TransactionType(req.Type))
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	if tx == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "transaction not found"})
		return
	}

	c.JSON(http.StatusOK, tx)
}

func (h *TransferHandler) GetMatches(c *gin.Context) {
	fromStr := c.Query("from")
	toStr := c.Query("to")

	if fromStr == "" || toStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "from and to are required"})
		return
	}

	from, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid from date, expected YYYY-MM-DD"})
		return
	}
	to, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid to date, expected YYYY-MM-DD"})
		return
	}

	var fxMin, fxMax *float64
	if minStr, maxStr := c.Query("fx_min"), c.Query("fx_max"); minStr != "" && maxStr != "" {
		min, err := strconv.ParseFloat(minStr, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid fx_min"})
			return
		}
		max, err := strconv.ParseFloat(maxStr, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid fx_max"})
			return
		}
		fxMin, fxMax = &min, &max
	}

	matches, err := h.svc.MatchForPeriod(c.Request.Context(), from, to, fxMin, fxMax)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to match transfers"})
		return
	}

	c.JSON(http.StatusOK, matches)
}
