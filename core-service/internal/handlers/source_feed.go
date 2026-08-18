package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/machineauth"
	"github.com/lease-management-system/core-service/internal/services/sourcefeed"
)

type SourceFeedHandler struct {
	credRepo *repository.MachineCredentialRepository
	kpiRepo  *repository.RetailKPIRepository
}

func NewSourceFeedHandler(
	credRepo *repository.MachineCredentialRepository,
	kpiRepo *repository.RetailKPIRepository,
) *SourceFeedHandler {
	return &SourceFeedHandler{
		credRepo: credRepo,
		kpiRepo:  kpiRepo,
	}
}

// ----------------------------------------------------------------------
// Machine Credentials Administration
// ----------------------------------------------------------------------

func (h *SourceFeedHandler) ListMachineCredentials(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	list, err := h.credRepo.ListCredentials(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if list == nil {
		list = []repository.MachineCredential{}
	}
	c.JSON(http.StatusOK, gin.H{"credentials": list})
}

type issueCredentialReq struct {
	Name      string   `json:"name" binding:"required"`
	Scopes    []string `json:"scopes"`
	ExpiresIn *int     `json:"expires_in_days"` // Days from now
}

func (h *SourceFeedHandler) IssueMachineCredential(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	var req issueCredentialReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.Scopes) == 0 {
		req.Scopes = []string{"operating_facts:write"}
	}

	var expiresAt *time.Time
	if req.ExpiresIn != nil && *req.ExpiresIn > 0 {
		exp := time.Now().AddDate(0, 0, *req.ExpiresIn)
		expiresAt = &exp
	}

	clientID, clientSecret, secretHash, err := machineauth.GenerateCredentials("pos")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate credentials"})
		return
	}

	cred := &repository.MachineCredential{
		LegalEntityID: tenantID,
		Name:          req.Name,
		ClientID:      clientID,
		SecretHash:    secretHash,
		Scopes:        req.Scopes,
		ExpiresAt:     expiresAt,
	}

	if err := h.credRepo.CreateCredential(c.Request.Context(), cred); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Return clientSecret ONCE on creation
	c.JSON(http.StatusCreated, gin.H{
		"id":            cred.ID,
		"name":          cred.Name,
		"client_id":     cred.ClientID,
		"client_secret": clientSecret, // Only returned at issue time
		"scopes":        cred.Scopes,
		"expires_at":    cred.ExpiresAt,
		"created_at":    cred.CreatedAt,
	})
}

func (h *SourceFeedHandler) RevokeMachineCredential(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	clientID := c.Param("id")

	if err := h.credRepo.RevokeCredential(c.Request.Context(), tenantID, clientID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "revoked", "client_id": clientID})
}

// ----------------------------------------------------------------------
// Push Feed Fact Ingestion Endpoint (PRD §3.F6-a)
// ----------------------------------------------------------------------

type PushFactsRequest struct {
	SourceSystem string                   `json:"source_system"`
	BatchID      string                   `json:"batch_id"`
	AsOf         string                   `json:"as_of"`
	Facts        []map[string]interface{} `json:"facts" binding:"required"`
}

func (h *SourceFeedHandler) PushFacts(c *gin.Context) {
	// 1. Authenticate Machine Credentials
	clientID := c.GetHeader("X-Client-ID")
	clientSecret := c.GetHeader("X-Client-Secret")

	if clientID == "" || clientSecret == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing X-Client-ID or X-Client-Secret header"})
		return
	}

	cred, err := h.credRepo.GetCredentialByClientID(c.Request.Context(), clientID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid machine credentials"})
		return
	}

	now := time.Now()
	authCred := &machineauth.Credential{
		ID:            cred.ID,
		LegalEntityID: cred.LegalEntityID,
		ClientID:      cred.ClientID,
		SecretHash:    cred.SecretHash,
		Scopes:        cred.Scopes,
		ExpiresAt:     cred.ExpiresAt,
		RevokedAt:     cred.RevokedAt,
	}

	if err := machineauth.Verify(authCred, clientSecret, "operating_facts:write", now); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	// Touch last used timestamp asynchronously
	go func() {
		_ = h.credRepo.TouchCredentialUsage(context.Background(), clientID)
	}()

	// 2. Parse Push Request
	var req PushFactsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.SourceSystem == "" {
		req.SourceSystem = "machine_api_push"
	}
	if req.BatchID == "" {
		req.BatchID = fmt.Sprintf("push_%s_%d", clientID, time.Now().Unix())
	}

	asOfTime := time.Now()
	if req.AsOf != "" {
		if t, err := time.Parse("2006-01-02", req.AsOf); err == nil {
			asOfTime = t
		}
	}

	env := sourcefeed.FeedEnvelope{
		SourceSystem:       req.SourceSystem,
		ImportBatchID:      req.BatchID,
		AsOfAt:             asOfTime,
		Version:            1,
		DataClassification: "production",
	}

	// 3. Convert via APIPushFeed Adapter
	feed := sourcefeed.NewAPIPushFeed(req.Facts, env)
	batch, err := feed.Fetch(c.Request.Context(), "")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":          "accepted",
		"batch_id":        batch.Envelope.ImportBatchID,
		"source_system":   batch.Envelope.SourceSystem,
		"records_count":   len(batch.Rows),
		"headers":         batch.Headers,
		"message":         "Facts batch received and mapped via SourceFeed adapter",
	})
}
