package handlers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/masterdataresolution"
)

type MasterDataResolutionHandler struct {
	db repository.DBTX
}

func NewMasterDataResolutionHandler(db repository.DBTX) *MasterDataResolutionHandler {
	return &MasterDataResolutionHandler{db: db}
}

// PersistentMappingReader implementation
type dbMappingReader struct {
	db            repository.DBTX
	legalEntityID string
	extSystem     string
}

func (r *dbMappingReader) GetConfirmedMappings(ctx context.Context, kind masterdataresolution.EntityKind, raws []string) (map[string]masterdataresolution.Candidate, error) {
	if len(raws) == 0 {
		return map[string]masterdataresolution.Candidate{}, nil
	}
	query := `
		SELECT raw_identifier, canonical_id, COALESCE(canonical_name, ''), confidence_score, resolved_by
		FROM master_data_entity_mappings
		WHERE legal_entity_id = $1 AND entity_kind = $2 AND is_confirmed = TRUE AND raw_identifier = ANY($3)
	`
	rows, err := r.db.Query(ctx, query, r.legalEntityID, string(kind), raws)
	if err != nil {
		return nil, fmt.Errorf("read confirmed mappings: %w", err)
	}
	defer rows.Close()

	out := make(map[string]masterdataresolution.Candidate)
	for rows.Next() {
		var raw, canonID, canonName, resolvedBy string
		var conf float64
		if err := rows.Scan(&raw, &canonID, &canonName, &conf, &resolvedBy); err == nil {
			out[raw] = masterdataresolution.Candidate{
				RawIdentifier: raw,
				CanonicalID:   canonID,
				CanonicalName: canonName,
				Confidence:    conf,
				Source:        "cached",
			}
		}
	}
	return out, nil
}

type resolveReq struct {
	Kind           string   `json:"kind" binding:"required"` // store, sku, category
	ExternalSystem string   `json:"external_system"`
	RawIdentifiers []string `json:"raw_identifiers" binding:"required"`
	Threshold      *float64 `json:"confidence_threshold"`
}

func (h *MasterDataResolutionHandler) Resolve(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	var req resolveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reader := &dbMappingReader{
		db:            h.db,
		legalEntityID: tenantID,
		extSystem:     req.ExternalSystem,
	}

	// Simple rule suggester (exact match / lowercase dictionary)
	dict := make(map[string]string)
	for _, raw := range req.RawIdentifiers {
		dict[raw] = raw // identity fallback rule
	}
	ruleSource := &masterdataresolution.RuleSuggestionSource{Dictionary: dict}

	threshold := 0.85
	if req.Threshold != nil && *req.Threshold > 0 {
		threshold = *req.Threshold
	}

	res, err := masterdataresolution.Resolve(
		c.Request.Context(),
		masterdataresolution.EntityKind(req.Kind),
		req.RawIdentifiers,
		reader,
		ruleSource,
		threshold,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

type confirmMappingReq struct {
	Kind           string  `json:"kind" binding:"required"`
	ExternalSystem string  `json:"external_system" binding:"required"`
	RawIdentifier  string  `json:"raw_identifier" binding:"required"`
	CanonicalID    string  `json:"canonical_id" binding:"required"`
	CanonicalName  *string `json:"canonical_name"`
	Confidence     float64 `json:"confidence_score"`
	ResolvedBy     string  `json:"resolved_by"`
}

func (h *MasterDataResolutionHandler) ConfirmMapping(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	var req confirmMappingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.ResolvedBy == "" {
		req.ResolvedBy = "manual"
	}
	if req.Confidence <= 0 {
		req.Confidence = 1.0
	}

	query := `
		INSERT INTO master_data_entity_mappings (
			legal_entity_id, entity_kind, external_system, raw_identifier,
			canonical_id, canonical_name, confidence_score, resolved_by, is_confirmed
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, TRUE)
		ON CONFLICT (legal_entity_id, entity_kind, external_system, raw_identifier)
		DO UPDATE SET
			canonical_id = EXCLUDED.canonical_id,
			canonical_name = EXCLUDED.canonical_name,
			confidence_score = EXCLUDED.confidence_score,
			resolved_by = EXCLUDED.resolved_by,
			is_confirmed = TRUE
	`
	_, err := h.db.Exec(
		c.Request.Context(), query,
		tenantID, req.Kind, req.ExternalSystem, req.RawIdentifier,
		req.CanonicalID, req.CanonicalName, req.Confidence, req.ResolvedBy,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "confirmed"})
}
