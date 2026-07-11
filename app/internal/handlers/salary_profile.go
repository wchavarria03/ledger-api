package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ledger-api/app/internal/auth"
	"ledger-api/app/internal/models"
)

func NewSalaryProfileHandler(svc SalaryProfileManager) *SalaryProfileHandler {
	return &SalaryProfileHandler{svc: svc}
}

func (h *SalaryProfileHandler) Get(c *gin.Context) {
	userID := auth.UserIDFromContext(c.Request.Context())
	profile, err := h.svc.Get(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if profile == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "salary profile not set"})
		return
	}
	c.JSON(http.StatusOK, profile)
}

func (h *SalaryProfileHandler) Save(c *gin.Context) {
	var req saveSalaryProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID := auth.UserIDFromContext(c.Request.Context())
	profile, err := h.svc.Save(c.Request.Context(), &models.SalaryProfile{
		UserID:       userID,
		NetSalary:    req.NetSalary,
		SalaryPeriod: req.SalaryPeriod,
		HoursPerWeek: req.HoursPerWeek,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, profile)
}

func (h *SalaryProfileHandler) CheckPurchase(c *gin.Context) {
	var req purchaseCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID := auth.UserIDFromContext(c.Request.Context())
	profile, err := h.svc.Get(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if profile == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "salary profile not set"})
		return
	}

	result, err := h.svc.CheckPurchase(c.Request.Context(), profile, req.Price)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
