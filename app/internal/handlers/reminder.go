package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"ledger-api/app/internal/models"
)

func NewReminderHandler(svc ReminderManager, transfers TransferService) *ReminderHandler {
	return &ReminderHandler{svc: svc, transfers: transfers}
}

func (h *ReminderHandler) List(c *gin.Context) {
	reminders, err := h.svc.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, reminders)
}

func (h *ReminderHandler) ListByAccount(c *gin.Context) {
	accountID := c.Param("id")
	reminders, err := h.svc.ListByAccountID(c.Request.Context(), accountID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, reminders)
}

func (h *ReminderHandler) Create(c *gin.Context) {
	var req createReminderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	input := models.ReminderInput{
		Title:   req.Title,
		DueDate: req.DueDate,
	}
	if req.AccountID != "" {
		input.AccountID = &req.AccountID
	}
	if req.Amount != nil {
		v := decimal.NewFromFloat(*req.Amount)
		input.Amount = &v
	}
	if req.Currency != "" {
		input.Currency = &req.Currency
	}
	if req.RecurrenceType != "" {
		input.RecurrenceType = &req.RecurrenceType
	}
	if req.Notes != "" {
		input.Notes = &req.Notes
	}

	reminder, err := h.svc.Create(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, reminder)
}

func (h *ReminderHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var req updateReminderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	fields := make(map[string]any)
	if req.AccountID != nil {
		fields["account_id"] = req.AccountID
	}
	if req.Title != "" {
		fields["title"] = req.Title
	}
	if req.Amount != nil {
		fields["amount"] = decimal.NewFromFloat(*req.Amount)
	}
	if req.Currency != "" {
		fields["currency"] = req.Currency
	}
	if req.DueDate != "" {
		fields["due_date"] = req.DueDate
	}
	if req.RecurrenceType != "" {
		fields["recurrence_type"] = req.RecurrenceType
	}
	if req.Notes != "" {
		fields["notes"] = req.Notes
	}
	if len(fields) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no fields to update"})
		return
	}

	reminder, err := h.svc.Update(c.Request.Context(), id, fields)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	if reminder == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "reminder not found"})
		return
	}
	c.JSON(http.StatusOK, reminder)
}

func (h *ReminderHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *ReminderHandler) Complete(c *gin.Context) {
	id := c.Param("id")
	var req completeReminderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.CreateTransfer {
		if req.FromAccountID == "" || req.ToAccountID == "" || req.Amount <= 0 || req.Currency == "" || req.Date == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "from_account_id, to_account_id, amount, currency, and date are required when create_transfer is true"})
			return
		}
		date, err := time.Parse("2006-01-02", req.Date)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date format, expected YYYY-MM-DD"})
			return
		}
		desc := req.Description
		if desc == "" {
			desc = fmt.Sprintf("Payment reminder: %s", id)
		}
		if _, err := h.transfers.CreateTransfer(c.Request.Context(), models.TransferInput{
			FromAccountID: req.FromAccountID,
			ToAccountID:   req.ToAccountID,
			Amount:        decimal.NewFromFloat(req.Amount),
			Currency:      req.Currency,
			Date:          date,
			Description:   desc,
		}); err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
	}

	reminder, err := h.svc.Complete(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	if reminder == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "reminder not found"})
		return
	}
	c.JSON(http.StatusOK, reminder)
}
