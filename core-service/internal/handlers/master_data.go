package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
)

type MasterDataHandler struct {
	repo *repository.MasterDataRepository
}

func NewMasterDataHandler(repo *repository.MasterDataRepository) *MasterDataHandler {
	return &MasterDataHandler{repo: repo}
}

func (h *MasterDataHandler) ListLegalEntities(c *gin.Context) {
	entities, err := h.repo.ListLegalEntities(c.Request.Context(), middleware.GetTenantID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch legal entities"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"legal_entities": entities})
}

func (h *MasterDataHandler) ListStores(c *gin.Context) {
	stores, err := h.repo.ListStores(
		c.Request.Context(),
		middleware.GetTenantID(c),
		c.Query("legal_entity_id"),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch stores"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"stores": stores})
}

func (h *MasterDataHandler) ListLandlords(c *gin.Context) {
	landlords, err := h.repo.ListLandlords(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch landlords"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"landlords": landlords})
}
