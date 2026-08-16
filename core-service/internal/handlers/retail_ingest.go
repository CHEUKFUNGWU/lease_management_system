package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/errcontract"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	auditservice "github.com/lease-management-system/core-service/internal/services/audit"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
	"github.com/lease-management-system/core-service/internal/services/retailingest"
)

// maxRetailIngestFileSize caps the controlled template upload; POS exports
// within one entity are far below this.
const maxRetailIngestFileSize = 10 << 20

type retailIngestPopulationReader interface {
	ListStorePopulation(context.Context, string, string, string, []string) ([]retailkpi.StorePopulation, error)
}

type retailIngestStore interface {
	retailStoreDayFactStore
	RetailStoreDayExistingState(context.Context, []string, []string, string) (map[string]int, error)
	CreateBatch(context.Context, *repository.OperatingFactBatch) (*repository.OperatingFactBatch, error)
	FinalizeBatch(context.Context, string, int, int, string, string, json.RawMessage) (*repository.OperatingFactBatch, error)
}

// retailIngestDirectory adapts the KPI store population (the stores master,
// entity-scoped) to the importer's StoreDirectory seam.
type retailIngestDirectory struct{ reader retailIngestPopulationReader }

func (d retailIngestDirectory) Stores(ctx context.Context, legalEntityID string) ([]retailingest.StoreRef, error) {
	population, err := d.reader.ListStorePopulation(ctx, legalEntityID, "production", "", nil)
	if err != nil {
		return nil, err
	}
	stores := make([]retailingest.StoreRef, 0, len(population))
	for _, store := range population {
		stores = append(stores, retailingest.StoreRef{StoreID: store.StoreID, StoreCode: store.StoreCode, StoreName: store.StoreName, Brand: store.Brand, Region: store.Region})
	}
	return stores, nil
}

// retailIngestSink adapts the atomic store-day writer (with entity scope,
// user and audit wiring captured per request) to the importer's FactSink
// seam. This is the second real adapter next to the test fake.
type retailIngestSink struct {
	store  retailIngestStore
	entity access.EntityFilter
	userID string
	audit  retailStoreDayFactAuditor
	ginCtx *gin.Context
}

func (s retailIngestSink) ExistingState(ctx context.Context, legalEntityID, sourceSystem string, storeIDs, businessDates []string) (map[string]int, error) {
	return s.store.RetailStoreDayExistingState(ctx, storeIDs, businessDates, sourceSystem)
}

func (s retailIngestSink) UpsertChunk(ctx context.Context, chunk []*repository.RetailStoreDayFact, idempotencyKey, payloadSHA256 string) (*repository.RetailStoreDayFactWriteResult, error) {
	createdBy := optionalString(s.userID)
	var auditFn repository.RetailStoreDayFactAuditFunc
	if s.audit != nil {
		auditFn = func(ctx context.Context, tx repository.DBTX, oldFact, newFact *repository.RetailStoreDayFact) error {
			return s.audit.LogInTx(ctx, tx, "retail_store_day_facts", newFact.ID, "import", oldFact, newFact, s.userID, s.ginCtx)
		}
	}
	return s.store.UpsertRetailStoreDayFactsAtomic(ctx, s.entity, chunk, idempotencyKey, payloadSHA256, createdBy, auditFn)
}

type RetailIngestHandler struct {
	population retailIngestPopulationReader
	store      retailIngestStore
	audit      retailStoreDayFactAuditor
	mappingAI  retailingest.MappingSuggester
}

func NewRetailIngestHandler(population retailIngestPopulationReader, store retailIngestStore, auditLogger any) *RetailIngestHandler {
	var auditor retailStoreDayFactAuditor
	switch value := auditLogger.(type) {
	case nil:
	case retailStoreDayFactAuditor:
		auditor = value
	case *auditservice.Logger:
		auditor = transactionalRetailStoreDayFactAuditor{logger: value}
	default:
		panic("unsupported retail ingest audit logger")
	}
	return &RetailIngestHandler{population: population, store: store, audit: auditor, mappingAI: NewRetailMappingAI()}
}

// Preview runs the deterministic pipeline without writing: parse, suggest
// (unless the client sends a corrected mapping), resolve stores, validate
// rows, and estimate overlap with existing facts.
func (h *RetailIngestHandler) Preview(c *gin.Context) {
	file, format, filename, err := retailIngestFile(c)
	if err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, err.Error(), nil)
		return
	}
	legalEntityID, ok := retailIngestScope(c)
	if !ok {
		return
	}
	sourceSystem := strings.TrimSpace(c.PostForm("source_system"))
	if sourceSystem == "" {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "source_system is required", nil)
		return
	}
	headers, rows, err := retailingest.ParseTemplate(file, format)
	if err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, err.Error(), nil)
		return
	}
	entity, _ := tenantEntity(c)
	service := retailingest.NewService(retailIngestDirectory{reader: h.population}, retailIngestSink{store: h.store, entity: entity, userID: userIDFromContext(c), audit: h.audit, ginCtx: c}).WithMappingSuggester(h.mappingAI)
	profiles := retailingest.ColumnProfiles(headers, rows)
	suggested, suggestionSource := service.SuggestMappingAssisted(c.Request.Context(), headers, profiles)
	mapping := suggested
	if raw := strings.TrimSpace(c.PostForm("mapping")); raw != "" {
		confirmed := retailingest.Mapping{}
		if err := json.Unmarshal([]byte(raw), &confirmed); err != nil {
			writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "mapping must be a JSON object of {column: field}", nil)
			return
		}
		mapping = confirmed
	}
	resolution, err := service.ResolveStores(c.Request.Context(), legalEntityID, mapping, headers, rows)
	if err != nil {
		writeSystemFailure(c, http.StatusInternalServerError, err)
		return
	}
	report := retailingest.Validate(headers, rows, mapping, resolution)
	coverage, err := service.EstimateOverlap(c.Request.Context(), legalEntityID, sourceSystem, report)
	if err != nil {
		writeSystemFailure(c, http.StatusInternalServerError, err)
		return
	}
	report.Coverage = coverage
	previewRows := rows
	if len(previewRows) > 8 {
		previewRows = previewRows[:8]
	}
	c.JSON(http.StatusOK, gin.H{
		"basis": "Working", "format": string(format), "source_file": filename,
		"standard_fields":   retailingest.AllFields,
		"headers":           headers,
		"column_profiles":   retailingest.ColumnProfiles(headers, rows),
		"suggested_mapping": suggested, "suggested_mapping_source": suggestionSource, "mapping": mapping, "rows_preview": previewRows,
		"resolution": gin.H{"matched_count": len(resolution.RawToStoreID), "unmatched": resolution.Unmatched},
		"report":     report,
	})
}

// Commit imports the file as production store-day facts: batch envelope,
// deterministic re-validation, superseding fact versions, chunked atomic
// writes with per-chunk idempotency derived from the Idempotency-Key.
func (h *RetailIngestHandler) Commit(c *gin.Context) {
	file, format, filename, err := retailIngestFile(c)
	if err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, err.Error(), nil)
		return
	}
	legalEntityID, ok := retailIngestScope(c)
	if !ok {
		return
	}
	sourceSystem := strings.TrimSpace(c.PostForm("source_system"))
	if sourceSystem == "" {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "source_system is required", nil)
		return
	}
	asOf, err := retailIngestAsOf(c.PostForm("as_of_at"))
	if err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, err.Error(), nil)
		return
	}
	idempotencyKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if idempotencyKey == "" {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "Idempotency-Key header is required for imports", nil)
		return
	}
	if len(idempotencyKey) > 255 {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "Idempotency-Key exceeds 255 characters", nil)
		return
	}
	headers, rows, err := retailingest.ParseTemplate(file, format)
	if err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, err.Error(), nil)
		return
	}
	mapping := retailingest.SuggestMapping(headers, retailingest.ColumnProfiles(headers, rows))
	if raw := strings.TrimSpace(c.PostForm("mapping")); raw != "" {
		confirmed := retailingest.Mapping{}
		if err := json.Unmarshal([]byte(raw), &confirmed); err != nil {
			writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "mapping must be a JSON object of {column: field}", nil)
			return
		}
		mapping = confirmed
	}
	entity, entityOK := tenantEntity(c)
	if !entityOK {
		writeCodedError(c, http.StatusForbidden, errcontract.CodePermissionDenied, "legal entity scope is required", nil)
		return
	}
	service := retailingest.NewService(retailIngestDirectory{reader: h.population}, retailIngestSink{store: h.store, entity: entity, userID: userIDFromContext(c), audit: h.audit, ginCtx: c})
	resolution, err := service.ResolveStores(c.Request.Context(), legalEntityID, mapping, headers, rows)
	if err != nil {
		writeSystemFailure(c, http.StatusInternalServerError, err)
		return
	}
	var legalEntityPayload *string
	if scopedID, idErr := entity.LegalEntityID(); idErr == nil {
		legalEntityPayload = &scopedID
	}
	batch, err := h.store.CreateBatch(c.Request.Context(), &repository.OperatingFactBatch{
		LegalEntityID: legalEntityPayload, SourceSystem: sourceSystem, SourceFile: filename,
		TotalRows: len(rows), AsOfAt: nowUTC(), CreatedBy: optionalString(userIDFromContext(c)),
		ReconciliationStatus: "unreconciled", IdempotencyKey: idempotencyKey,
		FactVersion: nowUTC().Format(time.RFC3339),
	})
	if err != nil {
		writeSystemFailure(c, http.StatusInternalServerError, err)
		return
	}
	if (batch.Status == "completed" || batch.Status == "failed") && batch.AcceptedRows+batch.RejectedRows > 0 {
		c.JSON(http.StatusOK, gin.H{
			"basis": "Working", "batch": batch, "saved_count": batch.AcceptedRows,
			"failed_count": batch.RejectedRows, "idempotent_replay": true,
		})
		return
	}
	envelope := retailingest.Envelope{SourceSystem: sourceSystem, ImportBatchID: batch.ID, AsOfAt: asOf}
	report, err := service.Commit(c.Request.Context(), legalEntityID, headers, rows, mapping, resolution, envelope, idempotencyKey)
	if err != nil {
		if errors.Is(err, retailingest.ErrEnvelopeIncomplete) {
			writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, err.Error(), nil)
			return
		}
		if errors.Is(err, retailingest.ErrNoValidRows) {
			_, _ = h.store.FinalizeBatch(c.Request.Context(), batch.ID, 0, report.RejectedRows, "failed", "unreconciled", retailIngestErrorsJSON(report))
			writeCodedError(c, http.StatusUnprocessableEntity, errcontract.CodeInvalidArguments, "every row failed validation", gin.H{"report": report, "batch_id": batch.ID})
			return
		}
		_, _ = h.store.FinalizeBatch(c.Request.Context(), batch.ID, report.AcceptedRows, report.RejectedRows, "failed", "unreconciled", retailIngestErrorsJSON(report))
		writeCodedFailure(c, http.StatusUnprocessableEntity, err, nil)
		return
	}
	batch, err = h.store.FinalizeBatch(c.Request.Context(), batch.ID, report.AcceptedRows, report.RejectedRows, "completed", "unreconciled", retailIngestErrorsJSON(report))
	if err != nil {
		writeSystemFailure(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"basis": "Working", "batch": batch, "report": report,
		"saved_count": report.AcceptedRows, "failed_count": report.RejectedRows,
		"idempotent_replay": report.ReplayDetected,
		"envelope":          gin.H{"source_system": sourceSystem, "import_batch_id": envelope.ImportBatchID, "as_of_at": envelope.AsOfAt},
	})
}

func retailIngestFile(c *gin.Context) ([]byte, retailingest.Format, string, error) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return nil, "", "", errors.New("file is required")
	}
	if fileHeader.Size > maxRetailIngestFileSize {
		return nil, "", "", errors.New("file exceeds the 10MB import limit")
	}
	opened, err := fileHeader.Open()
	if err != nil {
		return nil, "", "", errors.New("file cannot be opened")
	}
	defer opened.Close()
	data, err := io.ReadAll(opened)
	if err != nil {
		return nil, "", "", errors.New("file cannot be read")
	}
	format := retailingest.FormatCSV
	if strings.EqualFold(filepath.Ext(fileHeader.Filename), ".xlsx") {
		format = retailingest.FormatXLSX
	}
	return data, format, fileHeader.Filename, nil
}

func retailIngestScope(c *gin.Context) (string, bool) {
	legalEntityID := strings.TrimSpace(middleware.GetTenantID(c))
	if legalEntityID == "" {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "legal_entity_id is required", nil)
		return "", false
	}
	return legalEntityID, true
}

func retailIngestAsOf(raw string) (time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, errors.New("as_of_at is required")
	}
	if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return parsed.UTC(), nil
	}
	if parsed, err := time.Parse("2006-01-02", trimmed); err == nil {
		return parsed.UTC(), nil
	}
	return time.Time{}, errors.New("as_of_at must be RFC3339 or YYYY-MM-DD")
}

func retailIngestErrorsJSON(report *retailingest.ImportReport) json.RawMessage {
	if report == nil || len(report.Errors) == 0 {
		return json.RawMessage(`[]`)
	}
	encoded, err := json.Marshal(report.Errors)
	if err != nil {
		return json.RawMessage(`[]`)
	}
	return encoded
}
