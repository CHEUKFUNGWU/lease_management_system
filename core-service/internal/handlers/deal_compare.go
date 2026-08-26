package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/services/leasescenario"
)

// DealCompareHandler answers "which of these offers is cheaper" without
// touching stored data. The offers are hypothetical — terms on a table, not
// contracts — so nothing is read from or written to the ledger.
type DealCompareHandler struct{}

func NewDealCompareHandler() *DealCompareHandler { return &DealCompareHandler{} }

// Compare evaluates a set of proposed lease terms.
// POST /deals/compare
func (h *DealCompareHandler) Compare(c *gin.Context) {
	var input leasescenario.CompareInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := leasescenario.Compare(input)
	if err != nil {
		// The engine's messages already name what is missing or contradictory,
		// and they are written in the language the reader works in.
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}
