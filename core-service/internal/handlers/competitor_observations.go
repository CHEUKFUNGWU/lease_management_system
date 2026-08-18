package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/competitorreference"
)

type CompetitorObservationsHandler struct {
	repo *repository.CompetitorRepository
}

func NewCompetitorObservationsHandler(repo *repository.CompetitorRepository) *CompetitorObservationsHandler {
	return &CompetitorObservationsHandler{repo: repo}
}

func (h *CompetitorObservationsHandler) ListObservations(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	storeID := c.Query("store_id")

	obs, err := h.repo.ListObservations(c.Request.Context(), tenantID, storeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	summary := competitorreference.SummarizeStoreCompetitors(storeID, obs)
	c.JSON(http.StatusOK, gin.H{
		"benchmark":    summary,
		"observations": obs,
	})
}

type addCompetitorObsReq struct {
	StoreID          string   `json:"store_id" binding:"required"`
	CompetitorName   string   `json:"competitor_name" binding:"required"`
	CompetitorBrand  string   `json:"competitor_brand"`
	DistanceMeters   *int     `json:"distance_meters"`
	ObservationDate  string   `json:"observation_date" binding:"required"`
	PriceIndex       *float64 `json:"price_index"`
	PromoIntensity   string   `json:"promo_intensity"`
	FootfallEstimate *int     `json:"footfall_estimate"`
	Observer         string   `json:"observer"`
	Notes            string   `json:"notes"`
}

func (h *CompetitorObservationsHandler) AddObservation(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	var req addCompetitorObsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.PromoIntensity == "" {
		req.PromoIntensity = "medium"
	}

	obs := &competitorreference.Observation{
		LegalEntityID:    tenantID,
		StoreID:          req.StoreID,
		CompetitorName:   req.CompetitorName,
		CompetitorBrand:  req.CompetitorBrand,
		DistanceMeters:   req.DistanceMeters,
		ObservationDate:  req.ObservationDate,
		PriceIndex:       req.PriceIndex,
		PromoIntensity:   req.PromoIntensity,
		FootfallEstimate: req.FootfallEstimate,
		Observer:         req.Observer,
		Notes:            req.Notes,
	}

	if err := h.repo.AddObservation(c.Request.Context(), obs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, obs)
}
