package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"ledger-api/app/internal/models"
)

func NewBudgetHandler(budgets BudgetManager, transfers TransferService) *BudgetHandler {
	return &BudgetHandler{budgets: budgets, transfers: transfers}
}

func (h *BudgetHandler) List(c *gin.Context) {
	month := c.Query("month")
	if month == "" {
		month = time.Now().UTC().Format("2006-01")
	}
	statuses, err := h.budgets.List(c.Request.Context(), month)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, statuses)
}

func (h *BudgetHandler) Create(c *gin.Context) {
	var req createBudgetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	budget, err := h.budgets.Create(c.Request.Context(), models.BudgetInput{
		CategoryID: req.CategoryID,
		Currency:   req.Currency,
		Amount:     decimal.NewFromFloat(req.Amount),
	})
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, budget)
}

func (h *BudgetHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var req updateBudgetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	budget, err := h.budgets.Update(c.Request.Context(), id, decimal.NewFromFloat(req.Amount))
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	if budget == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "budget not found"})
		return
	}
	c.JSON(http.StatusOK, budget)
}

func (h *BudgetHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.budgets.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *BudgetHandler) Acknowledge(c *gin.Context) {
	id := c.Param("id")
	var req acknowledgeBudgetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var transferID *string
	if req.Action == "moved" {
		if req.FromAccountID == "" || req.ToAccountID == "" || req.Amount <= 0 || req.Currency == "" || req.Date == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "from_account_id, to_account_id, amount, currency, and date are required when action is 'moved'"})
			return
		}
		date, err := time.Parse("2006-01-02", req.Date)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date format, expected YYYY-MM-DD"})
			return
		}
		result, err := h.transfers.CreateTransfer(c.Request.Context(), models.TransferInput{
			FromAccountID: req.FromAccountID,
			ToAccountID:   req.ToAccountID,
			Amount:        decimal.NewFromFloat(req.Amount),
			Currency:      req.Currency,
			Date:          date,
			Description:   fmt.Sprintf("Budget underspend: %s", req.Month),
		})
		if err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
		transferID = &result.Transfer.ID
	}

	ack, err := h.budgets.Acknowledge(c.Request.Context(), id, req.Month, req.Action, transferID)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, ack)
}
