package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"ledger-api/app/internal/models"
)

func NewEnvelopeHandler(svc EnvelopeManager) *EnvelopeHandler {
	return &EnvelopeHandler{svc: svc}
}

func (h *EnvelopeHandler) List(c *gin.Context) {
	statuses, err := h.svc.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, statuses)
}

func (h *EnvelopeHandler) ListByAccount(c *gin.Context) {
	accountID := c.Param("id")
	statuses, err := h.svc.ListByAccountID(c.Request.Context(), accountID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, statuses)
}

func (h *EnvelopeHandler) Create(c *gin.Context) {
	var req createEnvelopeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	input := models.EnvelopeInput{
		AccountID:      req.AccountID,
		Name:           req.Name,
		Currency:       req.Currency,
		RecurrenceType: req.RecurrenceType,
	}
	if req.TargetAmount != nil {
		v := decimal.NewFromFloat(*req.TargetAmount)
		input.TargetAmount = &v
	}
	if req.RecurringAmount != nil {
		v := decimal.NewFromFloat(*req.RecurringAmount)
		input.RecurringAmount = &v
	}
	if req.NextContributionDate != "" {
		input.NextContributionDate = &req.NextContributionDate
	}

	status, err := h.svc.Create(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, status)
}

func (h *EnvelopeHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var req updateEnvelopeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	fields := make(map[string]any)
	if req.Name != "" {
		fields["name"] = req.Name
	}
	if req.TargetAmount != nil {
		fields["target_amount"] = decimal.NewFromFloat(*req.TargetAmount)
	}
	if req.RecurringAmount != nil {
		fields["recurring_amount"] = decimal.NewFromFloat(*req.RecurringAmount)
	}
	if req.RecurrenceType != "" {
		fields["recurrence_type"] = req.RecurrenceType
	}
	if req.NextContributionDate != "" {
		fields["next_contribution_date"] = req.NextContributionDate
	}
	if len(fields) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no fields to update"})
		return
	}

	status, err := h.svc.Update(c.Request.Context(), id, fields)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	if status == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "envelope not found"})
		return
	}
	c.JSON(http.StatusOK, status)
}

func (h *EnvelopeHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *EnvelopeHandler) Contribute(c *gin.Context) {
	id := c.Param("id")
	var req contributeEnvelopeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	input := models.ContributionInput{
		Amount:         decimal.NewFromFloat(req.Amount),
		Note:           req.Note,
		Date:           req.Date,
		ApplyRecurring: req.ApplyRecurring,
	}

	status, err := h.svc.Contribute(c.Request.Context(), id, input)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, status)
}
