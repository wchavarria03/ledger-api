package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ledger-api/app/internal/auth"
	"ledger-api/app/internal/models"
)

func NewAccountHandler(svc AccountLister) *AccountHandler {
	return &AccountHandler{svc: svc}
}

func (h *AccountHandler) List(c *gin.Context) {
	accounts, err := h.svc.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if externalParam := c.Query("external"); externalParam != "" {
		wantExternal := externalParam == "true"
		filtered := accounts[:0]
		for _, a := range accounts {
			if a.External == wantExternal {
				filtered = append(filtered, a)
			}
		}
		accounts = filtered
	}

	c.JSON(http.StatusOK, accounts)
}

func (h *AccountHandler) Create(c *gin.Context) {
	var req createAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID := auth.UserIDFromContext(c.Request.Context())
	account, err := h.svc.Create(c.Request.Context(), &models.Account{
		Name:          req.Name,
		BankName:      req.BankName,
		Currency:      req.Currency,
		AccountNumber: req.AccountNumber,
		UserID:        userID,
		Locked:        req.Locked,
		External:      req.External,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, account)
}

func (h *AccountHandler) Update(c *gin.Context) {
	var req updateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	fields := map[string]any{}
	if req.Alias != nil {
		fields["alias"] = *req.Alias
	}
	if req.Currency != "" {
		fields["currency"] = req.Currency
	}
	if req.Locked != nil {
		fields["locked"] = *req.Locked
	}
	if req.External != nil {
		fields["external"] = *req.External
	}
	if len(fields) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nothing to update"})
		return
	}
	account, err := h.svc.Update(c.Request.Context(), c.Param("id"), fields)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if account == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
		return
	}
	c.JSON(http.StatusOK, account)
}

func (h *AccountHandler) Get(c *gin.Context) {
	account, err := h.svc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if account == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
		return
	}
	c.JSON(http.StatusOK, account)
}
