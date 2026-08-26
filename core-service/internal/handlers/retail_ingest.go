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
	"github.com/lease-management-system/core-service/internal/services/retailingest"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
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
// retailIngestSink also satisfies retailingest.BatchStorer: the batch row and
// the facts share one store handle, as they did before the seam existed.
type retailIngestSink struct {
	store  retailIngestStore
	entity access.EntityFilter
	userID string
	audit  retailStoreDayFactAuditor
	ginCtx *gin.Context
}

func (s retailIngestSink) CreateBatch(ctx context.Context, batch *repository.OperatingFactBatch) (*repository.OperatingFactBatch, error) {
	return s.store.CreateBatch(ctx, batch)
}

func (s retailIngestSink) FinalizeBatch(ctx context.Context, id string, accepted, rejected int, status, reconciliation string, errorsJSON json.RawMessage) (*repository.OperatingFactBatch, error) {
	return s.store.FinalizeBatch(ctx, id, accepted, rejected, status, reconciliation, errorsJSON)
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
// Preview runs the deterministic pipeline without writing: parse, suggest
// (unless the client sends a corrected mapping), resolve stores, validate
// rows, and estimate overlap with existing facts. Orchestration lives behind
// the retailingest.PreviewBatch seam (C3); this handler keeps HTTP parsing,
// port wiring and error-code translation.
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
	entity, _ := tenantEntity(c)
	service := h.newService(entity, c)
	result, err := service.PreviewBatch(c.Request.Context(), retailingest.PreviewSpec{
		LegalEntityID: legalEntityID, Entity: entity, SourceSystem: sourceSystem,
		File: file, Format: format, RawMapping: c.PostForm("mapping"),
	})
	if err != nil {
		writeImportFailure(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"basis": "Working", "format": string(result.Format), "source_file": filename,
		"standard_fields":   result.StandardFields,
		"headers":           result.Headers,
		"column_profiles":   result.ColumnProfiles,
		"suggested_mapping": result.SuggestedMapping, "suggested_mapping_source": result.SuggestionSource, "mapping": result.Mapping, "rows_preview": result.RowsPreview,
		"resolution": gin.H{"matched_count": result.ResolutionMatched, "unmatched": result.ResolutionUnmatched},
		"report":     result.Report,
	})
}

// Commit imports the file as production store-day facts: batch envelope,
// deterministic re-validation, superseding fact versions, chunked atomic
// writes with per-chunk idempotency derived from the Idempotency-Key.
// The whole orchestration lives behind the retailingest.IngestBatch seam
// (C3); the handler parses HTTP parameters, wires the per-request ports and
// translates classified failures onto the error contract.
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
	entity, entityOK := tenantEntity(c)
	if !entityOK {
		writeCodedError(c, http.StatusForbidden, errcontract.CodePermissionDenied, "legal entity scope is required", nil)
		return
	}
	service := h.newService(entity, c)
	result, err := service.IngestBatch(c.Request.Context(), retailingest.IngestSpec{
		LegalEntityID: legalEntityID, Entity: entity, UserID: userIDFromContext(c),
		Filename: filename, File: file, Format: format,
		SourceSystem: sourceSystem, AsOf: asOf,
		IdempotencyKey: idempotencyKey, RawMapping: c.PostForm("mapping"),
	})
	if err != nil {
		writeImportFailure(c, err)
		return
	}
	if result.IdempotentReplay && result.Report == nil {
		c.JSON(http.StatusOK, gin.H{
			"basis": "Working", "batch": result.Batch, "saved_count": result.SavedCount,
			"failed_count": result.FailedCount, "idempotent_replay": true,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"basis": "Working", "batch": result.Batch, "report": result.Report,
		"saved_count": result.SavedCount, "failed_count": result.FailedCount,
		"idempotent_replay": result.IdempotentReplay,
		"envelope":          gin.H{"source_system": sourceSystem, "import_batch_id": result.Envelope.ImportBatchID, "as_of_at": result.Envelope.AsOfAt},
	})
}

// newService wires the per-request adapters: the store population as the
// master-data directory, the atomic store-day writer (with entity scope, user
// and audit captured) as sink and batch store.
func (h *RetailIngestHandler) newService(entity access.EntityFilter, c *gin.Context) *retailingest.Service {
	sink := retailIngestSink{store: h.store, entity: entity, userID: userIDFromContext(c), audit: h.audit, ginCtx: c}
	return retailingest.NewService(retailIngestDirectory{reader: h.population}, sink).WithMappingSuggester(h.mappingAI).WithBatchStore(sink)
}

// writeImportFailure is the single failure→HTTP translation for the ingest
// seam: kinds map onto status/code families, messages pass through verbatim.
func writeImportFailure(c *gin.Context, err error) {
	var imp *retailingest.ImportError
	if !errors.As(err, &imp) {
		writeSystemFailure(c, http.StatusInternalServerError, err)
		return
	}
	switch imp.Kind {
	case retailingest.FailureParse, retailingest.FailureEnvelope:
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, imp.Message, nil)
	case retailingest.FailureNoValidRows:
		writeCodedError(c, http.StatusUnprocessableEntity, errcontract.CodeInvalidArguments, imp.Message, gin.H{"report": imp.Report, "batch_id": imp.BatchID})
	case retailingest.FailureCommit:
		writeCodedFailure(c, http.StatusUnprocessableEntity, imp.Err, nil)
	default:
		writeSystemFailure(c, http.StatusInternalServerError, imp.Err)
	}
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
