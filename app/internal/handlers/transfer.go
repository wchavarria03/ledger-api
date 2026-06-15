package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func NewTransferHandler(matcher TransferMatcher) *TransferHandler {
	return &TransferHandler{matcher: matcher}
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

	matches, err := h.matcher.MatchForPeriod(c.Request.Context(), from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to match transfers"})
		return
	}

	c.JSON(http.StatusOK, matches)
}
