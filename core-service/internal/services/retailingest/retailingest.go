// Package retailingest turns POS / finance export files into controlled
// production store-day facts (PRD P5-1, design M8). The four-stage contract —
// ParseTemplate, SuggestMapping, ResolveStores, Validate, Commit — hides
// header normalization, store identity resolution, deterministic numeric
// parsing, versioned correction, chunking and idempotency behind one module.
//
// Discipline encoded here (AGENTS.md bottom lines): every fact carries the
// full source envelope (source_system / import_batch_id / as_of_at — all
// three mandatory), only production facts are produced (the simulator owns
// simulated data), corrections supersede by a new fact version instead of
// mutating history, and LLM mapping suggestions — when an assist adapter is
// added later — only ever produce Mapping proposals confirmed by a human;
// numbers never enter any model.
package retailingest

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/controlledxlsx"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/controlledintake"
)

// MaxChunkRows mirrors the store-day JSON API batch ceiling; Commit splits
// larger files into chunks of this size.
const MaxChunkRows = 500

// Format identifies the controlled template container.
type Format string

const (
	FormatCSV  Format = "csv"
	FormatXLSX Format = "xlsx"
)

// Standard field targets of a Mapping (file column header → field).
const (
	FieldStore                 = "store"
	FieldBusinessDate          = "business_date"
	FieldCurrency              = "currency"
	FieldRevenue               = "revenue"
	FieldGrossProfit           = "gross_profit"
	FieldTransactions          = "transactions"
	FieldFootfall              = "footfall"
	FieldAreaSqm               = "area_sqm"
	FieldLaborCost             = "labor_cost"
	FieldFixedRent             = "fixed_rent"
	FieldVariableRent          = "variable_rent"
	FieldNonLeaseCost          = "non_lease_cost"
	FieldOtherControllableCost = "other_controllable_cost"
)

// RequiredFields must all be mapped before any row can validate.
var RequiredFields = []string{FieldStore, FieldBusinessDate, FieldCurrency, FieldRevenue}

// NumericFields are optional metrics parsed as non-negative decimals.
var NumericFields = []string{
	FieldGrossProfit, FieldTransactions, FieldFootfall, FieldAreaSqm, FieldLaborCost,
	FieldFixedRent, FieldVariableRent, FieldNonLeaseCost, FieldOtherControllableCost,
}

// AllFields is the single option list for mapping UIs and the contract-test
// source of truth (CONTRACT-001 shape: backend whitelist, frontend consumes).
var AllFields = func() []string {
	fields := make([]string, 0, len(RequiredFields)+len(NumericFields))
	fields = append(fields, RequiredFields...)
	fields = append(fields, NumericFields...)
	return fields
}()

// Mapping maps a file column header (exact, trimmed) to a standard field.
type Mapping map[string]string

// Envelope is the mandatory provenance triple for a commit.
type Envelope struct {
	SourceSystem  string    `json:"source_system"`
	ImportBatchID string    `json:"import_batch_id"`
	AsOfAt        time.Time `json:"as_of_at"`
}

// Validate refuses an envelope missing any of the three provenance fields —
// traceability is not optional (bottom line 3).
func (e Envelope) Validate() error {
	missing := make([]string, 0, 3)
	if strings.TrimSpace(e.SourceSystem) == "" {
		missing = append(missing, "source_system")
	}
	if strings.TrimSpace(e.ImportBatchID) == "" {
		missing = append(missing, "import_batch_id")
	}
	if e.AsOfAt.IsZero() {
		missing = append(missing, "as_of_at")
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: missing %s", ErrEnvelopeIncomplete, strings.Join(missing, ", "))
	}
	return nil
}

// ErrEnvelopeIncomplete signals a commit refused for provenance reasons.
var ErrEnvelopeIncomplete = errors.New("import envelope incomplete")

// ErrNoValidRows signals a commit where every row failed validation.
var ErrNoValidRows = errors.New("no valid rows to import")

// StoreRef is one store-master entry for identity resolution.
type StoreRef struct {
	StoreID   string `json:"store_id"`
	StoreCode string `json:"store_code"`
	StoreName string `json:"store_name"`
	Brand     string `json:"brand,omitempty"`
	Region    string `json:"region,omitempty"`
}

// StoreDirectory supplies the entity-scoped store master. The repository
// adapter reads the stores table (not fact-derived), so brand-new stores
// without any facts are resolvable.
type StoreDirectory interface {
	Stores(ctx context.Context, legalEntityID string) ([]StoreRef, error)
}

// FactSink is the write adapter owned by the handler wiring (repository +
// entity scope + user + audit); tests provide the second implementation,
// which makes this a real seam.
type FactSink interface {
	// ExistingState returns the current max fact version per
	// "store_id|business_date" key for the source system; 0 means absent.
	ExistingState(ctx context.Context, legalEntityID, sourceSystem string, storeIDs, businessDates []string) (map[string]int, error)
	UpsertChunk(ctx context.Context, chunk []*repository.RetailStoreDayFact, idempotencyKey, payloadSHA256 string) (*repository.RetailStoreDayFactWriteResult, error)
}

// ColumnProfile describes one file column for mapping UIs and (later) the
// LLM assist adapter: counts only, plus a digit-masked sample — never raw
// numbers.
type ColumnProfile struct {
	Header       string `json:"header"`
	NonEmpty     int    `json:"non_empty"`
	Numeric      int    `json:"numeric_like"`
	DateLike     int    `json:"date_like"`
	MaskedSample string `json:"masked_sample,omitempty"`
}

// StoreResolution is the outcome of matching the file's store column against
// the store master.
type StoreResolution struct {
	RawToStoreID map[string]string   `json:"-"`
	StoreByID    map[string]StoreRef `json:"-"`
	Unmatched    []string            `json:"unmatched"`
}

// Resolved reports whether a raw identifier resolved to a store.
func (r StoreResolution) Resolved(raw string) (string, bool) {
	id, ok := r.RawToStoreID[normalizeKey(raw)]
	return id, ok
}

// RowError is the single row-level contract shared by every importer
// (controlledintake); the Column field is the store-day importer's extra
// context. Kept as an alias so callers and tests crossing this seam keep
// one shape.
type RowError = controlledintake.RowError

// CoverageEstimate previews the overlap of a file with existing facts.
type CoverageEstimate struct {
	StoreCount       int    `json:"store_count"`
	DateFrom         string `json:"date_from,omitempty"`
	DateTo           string `json:"date_to,omitempty"`
	OverlapStoreDays int    `json:"overlap_store_days"`
	NewStoreDays     int    `json:"new_store_days"`
}

// ValidationReport is the full pre-commit verdict.
type ValidationReport struct {
	TotalRows         int              `json:"total_rows"`
	ValidRows         int              `json:"valid_rows"`
	Errors            []RowError       `json:"errors,omitempty"`
	UnmatchedStores   []string         `json:"unmatched_stores,omitempty"`
	MissingFields     []string         `json:"missing_fields,omitempty"`
	AmbiguousMappings []string         `json:"ambiguous_mappings,omitempty"`
	Coverage          CoverageEstimate `json:"coverage"`
	Facts             []ParsedFact     `json:"-"`
}

// ParsedFact is a validated row ready for versioning and commit.
type ParsedFact struct {
	StoreID               string
	BusinessDate          string
	Currency              string
	Revenue               float64
	GrossProfit           *float64
	Transactions          *float64
	Footfall              *float64
	AreaSqm               *float64
	LaborCost             *float64
	FixedRent             *float64
	VariableRent          *float64
	NonLeaseCost          *float64
	OtherControllableCost *float64
	SourceRecordID        string
	RowIndex              int
}

// ImportReport is the Commit outcome.
type ImportReport struct {
	BatchID             string     `json:"batch_id"`
	TotalRows           int        `json:"total_rows"`
	AcceptedRows        int        `json:"accepted_rows"`
	RejectedRows        int        `json:"rejected_rows"`
	Chunks              int        `json:"chunks"`
	ChunkSize           int        `json:"chunk_size"`
	ReplayDetected      bool       `json:"replay_detected"`
	NewStoreDays        int        `json:"new_store_days"`
	SupersededStoreDays int        `json:"superseded_store_days"`
	Errors              []RowError `json:"errors,omitempty"`
}

// Service owns the import pipeline behind the directory and sink adapters.
type Service struct {
	directory   StoreDirectory
	sink        FactSink
	batches     BatchStorer
	aiSuggester MappingSuggester
}

// NewService builds the importer; both adapters are required.
func NewService(directory StoreDirectory, sink FactSink) *Service {
	return &Service{directory: directory, sink: sink}
}

// WithMappingSuggester attaches the AI assist adapter (Assist Mode); the
// deterministic table remains the fallback whenever it is absent or fails.
func (s *Service) WithMappingSuggester(suggester MappingSuggester) *Service {
	s.aiSuggester = suggester
	return s
}

// WithBatchStore attaches the batch-lifecycle port (C3, 架构重构任务书
// 2026-08-26)。IngestBatch 需要它创建/收尾 operating_fact_batches 行；
// 不接则 IngestBatch 以 system 失败诚实拒绝，不产出任何行。

// SuggestMappingAssisted returns the AI proposal when the adapter is
// attached and succeeds; otherwise the deterministic suggestion. The source
// ("ai" | "rule") is surfaced to the UI so the human knows what they are
// confirming.
func (s *Service) SuggestMappingAssisted(ctx context.Context, headers []string, columnProfiles []ColumnProfile) (Mapping, string) {
	if s.aiSuggester != nil {
		if aiMapping, err := s.aiSuggester.SuggestMapping(ctx, headers, columnProfiles); err == nil && len(aiMapping) > 0 {
			// The AI proposal only fills what the deterministic table
			// missed; alias hits stay deterministic.
			ruleMapping := SuggestMapping(headers, columnProfiles)
			merged := Mapping{}
			for header, field := range aiMapping {
				merged[header] = field
			}
			for header, field := range ruleMapping {
				if _, exists := merged[header]; !exists {
					merged[header] = field
				}
			}
			return merged, "ai"
		}
	}
	return SuggestMapping(headers, columnProfiles), "rule"
}

var dateLayouts = []string{"2006-01-02", "2006/01/02", "2006/1/2", "2006-1-2"}

var currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

// ParseTemplate reads the controlled template container and splits header
// from data rows. Whitespace is trimmed; fully empty rows and trailing empty
// columns are dropped.
func ParseTemplate(file []byte, format Format) ([]string, [][]string, error) {
	var grid [][]string
	switch format {
	case FormatCSV:
		reader := newTrimmingCSVReader(file)
		raw, err := reader.ReadAll()
		if err != nil {
			return nil, nil, fmt.Errorf("invalid CSV: %w", err)
		}
		grid = raw
	case FormatXLSX:
		raw, err := controlledxlsx.Read(file)
		if err != nil {
			return nil, nil, err
		}
		grid = raw
	default:
		return nil, nil, fmt.Errorf("unsupported template format %q", format)
	}
	if len(grid) == 0 {
		return nil, nil, errors.New("template contains no header row")
	}
	trimmed := make([][]string, 0, len(grid))
	for _, row := range grid {
		values := make([]string, len(row))
		for i, value := range row {
			values[i] = strings.TrimSpace(value)
		}
		trimmed = append(trimmed, values)
	}
	headerRow := trimmed[0]
	lastMeaningful := -1
	for i, header := range headerRow {
		if header != "" {
			lastMeaningful = i
		}
	}
	if lastMeaningful < 0 {
		return nil, nil, errors.New("template header row is empty")
	}
	headers := headerRow[:lastMeaningful+1]
	rows := make([][]string, 0, len(trimmed)-1)
	for _, row := range trimmed[1:] {
		if rowIsEmpty(row) {
			continue
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, nil, errors.New("template contains no data rows")
	}
	return headers, rows, nil
}

func rowIsEmpty(row []string) bool {
	for _, value := range row {
		if value != "" {
			return false
		}
	}
	return true
}

// newTrimmingCSVReader reads CSV bytes with per-field whitespace trimming and
// variable record lengths (short rows are padded by cellAt).
func newTrimmingCSVReader(file []byte) *csv.Reader {
	reader := csv.NewReader(bytes.NewReader(file))
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true
	return reader
}

// ColumnProfiles summarizes each column: counts plus a digit-masked sample
// (numbers never leave the deterministic path in raw form).
func ColumnProfiles(headers []string, rows [][]string) []ColumnProfile {
	profiles := make([]ColumnProfile, len(headers))
	for index, header := range headers {
		profile := ColumnProfile{Header: header}
		for _, row := range rows {
			value := cellAt(row, index)
			if value == "" {
				continue
			}
			profile.NonEmpty++
			if _, err := parseAmount(value); err == nil {
				profile.Numeric++
			}
			if _, err := parseBusinessDate(value); err == nil {
				profile.DateLike++
			}
			if profile.MaskedSample == "" {
				profile.MaskedSample = maskDigits(value)
			}
		}
		profiles[index] = profile
	}
	return profiles
}

// SuggestMapping proposes file columns for the standard fields using the
// deterministic alias table plus the column profiles (date-like columns and
// numeric columns bias their target fields when the alias table misses). The
// proposal is Assist Mode material: the UI confirms or corrects it before
// validate/commit, and the AI assist adapter may refine it — suggestions
// alone never touch data, and raw values never leave Go.
func SuggestMapping(headers []string, columnProfiles []ColumnProfile) Mapping {
	mapping := Mapping{}
	profileByHeader := map[string]ColumnProfile{}
	for _, profile := range columnProfiles {
		profileByHeader[profile.Header] = profile
	}
	for _, header := range headers {
		normalized := normalizeKey(header)
		if _, exists := mapping[normalized]; exists {
			continue
		}
		if field, ok := headerAliases[normalized]; ok {
			mapping[header] = field
			continue
		}
		// Profile-driven fallback: an unrecognized header that is strongly
		// date-like or numeric still gets a sensible suggestion.
		profile := profileByHeader[header]
		if profile.NonEmpty > 0 && profile.DateLike > 0 && profile.DateLike >= profile.Numeric {
			mapping[header] = FieldBusinessDate
		}
	}
	return mapping
}

// MappingSuggester is the AI assist adapter (second adapter next to the
// deterministic table): it only produces Mapping proposals — the human
// confirms, and deterministic parsing takes over afterwards.
type MappingSuggester interface {
	SuggestMapping(ctx context.Context, headers []string, columnProfiles []ColumnProfile) (Mapping, error)
}

// ResolveStores matches the mapped store column against the entity's store
// master by UUID, store code, then unique name. Values that match nothing —
// including stores of another legal entity — land in Unmatched and become
// row errors in Validate, never silent drops.
func (s *Service) ResolveStores(ctx context.Context, legalEntityID string, mapping Mapping, headers []string, rows [][]string) (StoreResolution, error) {
	stores, err := s.directory.Stores(ctx, legalEntityID)
	if err != nil {
		return StoreResolution{}, err
	}
	resolution := StoreResolution{RawToStoreID: map[string]string{}, StoreByID: map[string]StoreRef{}}
	byCode, byName := map[string][]StoreRef{}, map[string][]StoreRef{}
	for _, store := range stores {
		resolution.StoreByID[store.StoreID] = store
		if code := normalizeKey(store.StoreCode); code != "" {
			byCode[code] = append(byCode[code], store)
		}
		if name := normalizeKey(store.StoreName); name != "" {
			byName[name] = append(byName[name], store)
		}
	}
	rawValues := map[string]bool{}
	storeColumn := columnOf(mapping, headers, FieldStore)
	if storeColumn >= 0 {
		for _, row := range rows {
			if value := cellAt(row, storeColumn); value != "" {
				rawValues[value] = true
			}
		}
	}
	keys := make([]string, 0, len(rawValues))
	for key := range rawValues {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, raw := range keys {
		normalized := normalizeKey(raw)
		resolved := ""
		switch {
		case isUUID(raw):
			if _, ok := resolution.StoreByID[strings.ToLower(raw)]; ok {
				resolved = strings.ToLower(raw)
			}
		case len(byCode[normalized]) == 1:
			resolved = byCode[normalized][0].StoreID
		case len(byName[normalized]) == 1:
			resolved = byName[normalized][0].StoreID
		}
		if resolved != "" {
			resolution.RawToStoreID[normalized] = resolved
		} else {
			resolution.Unmatched = append(resolution.Unmatched, raw)
		}
	}
	return resolution, nil
}

// Validate runs the full deterministic row pipeline and returns the report
// (with the parsed facts attached for the caller's coverage estimate and
// commit). It is pure: no sink, no DB.
func Validate(headers []string, rows [][]string, mapping Mapping, resolution StoreResolution) ValidationReport {
	report := ValidationReport{TotalRows: len(rows), Facts: make([]ParsedFact, 0, len(rows))}
	columns := map[string][]string{}
	for header, field := range mapping {
		if field != "" {
			columns[field] = append(columns[field], header)
		}
	}
	for field, headersForField := range columns {
		if len(headersForField) > 1 {
			sort.Strings(headersForField)
			report.AmbiguousMappings = append(report.AmbiguousMappings, field+"="+strings.Join(headersForField, "|"))
		}
	}
	sort.Strings(report.AmbiguousMappings)
	for _, field := range RequiredFields {
		if len(columns[field]) == 0 {
			report.MissingFields = append(report.MissingFields, field)
		}
	}
	positions := map[string]int{}
	for _, field := range AllFields {
		positions[field] = columnOf(mapping, headers, field)
	}
	seenStoreDays := map[string]bool{}
	for rowIndex, row := range rows {
		fact, rowErr := validateRow(row, rowIndex, positions, resolution)
		if rowErr != nil {
			report.Errors = append(report.Errors, *rowErr)
			continue
		}
		storeDay := fact.StoreID + "|" + fact.BusinessDate
		if seenStoreDays[storeDay] {
			report.Errors = append(report.Errors, RowError{Row: rowIndex + 1, Code: "duplicate_in_file", Message: "duplicate (store, business_date) within the file"})
			continue
		}
		seenStoreDays[storeDay] = true
		report.Facts = append(report.Facts, *fact)
	}
	report.UnmatchedStores = resolution.Unmatched
	report.ValidRows = len(report.Facts)
	return report
}

func validateRow(row []string, rowIndex int, positions map[string]int, resolution StoreResolution) (*ParsedFact, *RowError) {
	fact := &ParsedFact{RowIndex: rowIndex}
	rawStore := valueAt(row, positions[FieldStore])
	if rawStore == "" {
		return nil, &RowError{Row: rowIndex + 1, Column: headerFor(positions[FieldStore]), Code: "missing_required", Message: "store is required"}
	}
	storeID, ok := resolution.Resolved(rawStore)
	if !ok {
		return nil, &RowError{Row: rowIndex + 1, Column: headerFor(positions[FieldStore]), Code: "unmatched_store", Message: fmt.Sprintf("store %q is not in this legal entity's store master", rawStore)}
	}
	fact.StoreID = storeID
	rawDate := valueAt(row, positions[FieldBusinessDate])
	if rawDate == "" {
		return nil, &RowError{Row: rowIndex + 1, Column: headerFor(positions[FieldBusinessDate]), Code: "missing_required", Message: "business_date is required"}
	}
	businessDate, err := parseBusinessDate(rawDate)
	if err != nil {
		return nil, &RowError{Row: rowIndex + 1, Column: headerFor(positions[FieldBusinessDate]), Code: "bad_date", Message: fmt.Sprintf("business_date %q must be YYYY-MM-DD or YYYY/MM/DD", rawDate)}
	}
	fact.BusinessDate = businessDate
	rawCurrency := strings.ToUpper(valueAt(row, positions[FieldCurrency]))
	if rawCurrency == "" {
		return nil, &RowError{Row: rowIndex + 1, Column: headerFor(positions[FieldCurrency]), Code: "missing_required", Message: "currency is required"}
	}
	if !currencyPattern.MatchString(rawCurrency) {
		return nil, &RowError{Row: rowIndex + 1, Column: headerFor(positions[FieldCurrency]), Code: "bad_currency", Message: fmt.Sprintf("currency %q must be a three-letter ISO code", rawCurrency)}
	}
	fact.Currency = rawCurrency
	rawRevenue := valueAt(row, positions[FieldRevenue])
	if rawRevenue == "" {
		return nil, &RowError{Row: rowIndex + 1, Column: headerFor(positions[FieldRevenue]), Code: "missing_required", Message: "revenue is required; use 0 explicitly for a confirmed zero"}
	}
	revenue, err := parseAmount(rawRevenue)
	if err != nil {
		return nil, &RowError{Row: rowIndex + 1, Column: headerFor(positions[FieldRevenue]), Code: "bad_number", Message: fmt.Sprintf("revenue %q is not a number", rawRevenue)}
	}
	if revenue < 0 {
		return nil, &RowError{Row: rowIndex + 1, Column: headerFor(positions[FieldRevenue]), Code: "negative_value", Message: "revenue cannot be negative"}
	}
	fact.Revenue = revenue
	numericTargets := map[string]**float64{
		FieldGrossProfit: &fact.GrossProfit, FieldTransactions: &fact.Transactions, FieldFootfall: &fact.Footfall,
		FieldAreaSqm: &fact.AreaSqm, FieldLaborCost: &fact.LaborCost, FieldFixedRent: &fact.FixedRent,
		FieldVariableRent: &fact.VariableRent, FieldNonLeaseCost: &fact.NonLeaseCost, FieldOtherControllableCost: &fact.OtherControllableCost,
	}
	for _, field := range NumericFields {
		raw := valueAt(row, positions[field])
		if raw == "" {
			continue
		}
		amount, err := parseAmount(raw)
		if err != nil {
			return nil, &RowError{Row: rowIndex + 1, Column: headerFor(positions[field]), Code: "bad_number", Message: fmt.Sprintf("%s %q is not a number", field, raw)}
		}
		if amount < 0 {
			return nil, &RowError{Row: rowIndex + 1, Column: headerFor(positions[field]), Code: "negative_value", Message: field + " cannot be negative"}
		}
		*numericTargets[field] = &amount
	}
	fact.SourceRecordID = fmt.Sprintf("row:%d", rowIndex+1)
	return fact, nil
}

// EstimateOverlap previews how the validated facts sit against existing
// production facts for the same source system (the sink-side half of the
// validation report).
func (s *Service) EstimateOverlap(ctx context.Context, legalEntityID, sourceSystem string, report ValidationReport) (CoverageEstimate, error) {
	coverage := report.Coverage
	stores, dates := map[string]bool{}, map[string]bool{}
	storeIDs, businessDates := make([]string, 0), make([]string, 0)
	for _, fact := range report.Facts {
		if !stores[fact.StoreID] {
			stores[fact.StoreID] = true
			storeIDs = append(storeIDs, fact.StoreID)
		}
		if !dates[fact.BusinessDate] {
			dates[fact.BusinessDate] = true
			businessDates = append(businessDates, fact.BusinessDate)
		}
	}
	coverage.StoreCount = len(stores)
	if len(report.Facts) > 0 {
		ordered := make([]string, 0, len(dates))
		for date := range dates {
			ordered = append(ordered, date)
		}
		sort.Strings(ordered)
		coverage.DateFrom, coverage.DateTo = ordered[0], ordered[len(ordered)-1]
	}
	if len(storeIDs) == 0 {
		return coverage, nil
	}
	state, err := s.sink.ExistingState(ctx, legalEntityID, sourceSystem, storeIDs, businessDates)
	if err != nil {
		return CoverageEstimate{}, err
	}
	for _, fact := range report.Facts {
		if state[fact.StoreID+"|"+fact.BusinessDate] > 0 {
			coverage.OverlapStoreDays++
		} else {
			coverage.NewStoreDays++
		}
	}
	return coverage, nil
}

// Commit re-validates deterministically (the client's report is never
// trusted), assigns superseding fact versions, chunks, and writes through
// the sink with per-chunk idempotency derived from the caller's key.
func (s *Service) Commit(ctx context.Context, legalEntityID string, headers []string, rows [][]string, mapping Mapping, resolution StoreResolution, envelope Envelope, idempotencyKey string) (*ImportReport, error) {
	if err := envelope.Validate(); err != nil {
		return nil, err
	}
	report := Validate(headers, rows, mapping, resolution)
	importReport := &ImportReport{
		BatchID: envelope.ImportBatchID, TotalRows: report.TotalRows,
		RejectedRows: len(report.Errors), Errors: report.Errors, ChunkSize: MaxChunkRows,
	}
	if report.ValidRows == 0 {
		return importReport, ErrNoValidRows
	}
	storeIDs, businessDates := collectPairs(report.Facts)
	state, err := s.sink.ExistingState(ctx, legalEntityID, envelope.SourceSystem, storeIDs, businessDates)
	if err != nil {
		return nil, err
	}
	for chunkStart := 0; chunkStart < len(report.Facts); chunkStart += MaxChunkRows {
		chunkEnd := chunkStart + MaxChunkRows
		if chunkEnd > len(report.Facts) {
			chunkEnd = len(report.Facts)
		}
		chunk := make([]*repository.RetailStoreDayFact, 0, chunkEnd-chunkStart)
		for _, fact := range report.Facts[chunkStart:chunkEnd] {
			key := fact.StoreID + "|" + fact.BusinessDate
			importBatchID := envelope.ImportBatchID
			chunk = append(chunk, &repository.RetailStoreDayFact{
				StoreID: fact.StoreID, BusinessDate: fact.BusinessDate, Currency: fact.Currency, Revenue: fact.Revenue,
				GrossProfit: fact.GrossProfit, Transactions: fact.Transactions, Footfall: fact.Footfall,
				AreaSqm: fact.AreaSqm, LaborCost: fact.LaborCost, FixedRent: fact.FixedRent,
				VariableRent: fact.VariableRent, NonLeaseCost: fact.NonLeaseCost,
				OtherControllableCost: fact.OtherControllableCost, SourceSystem: envelope.SourceSystem,
				SourceRecordID: fact.SourceRecordID, ImportBatchID: &importBatchID,
				AsOfAt: envelope.AsOfAt.UTC(), Version: state[key] + 1,
				MappingStatus: "mapped", DataQualityStatus: "valid", DataClassification: "production",
			})
			if state[key] > 0 {
				importReport.SupersededStoreDays++
			} else {
				importReport.NewStoreDays++
			}
		}
		payload, err := json.Marshal(chunk)
		if err != nil {
			return nil, fmt.Errorf("encode chunk payload: %w", err)
		}
		digest := sha256.Sum256(payload)
		chunkKey := ""
		if strings.TrimSpace(idempotencyKey) != "" {
			chunkKey = strings.TrimSpace(idempotencyKey) + "#chunk:" + strconv.Itoa(importReport.Chunks)
		}
		result, err := s.sink.UpsertChunk(ctx, chunk, chunkKey, hex.EncodeToString(digest[:]))
		if err != nil {
			return nil, err
		}
		importReport.AcceptedRows += len(result.Facts)
		if result.Replayed {
			importReport.ReplayDetected = true
		}
		importReport.Chunks++
	}
	return importReport, nil
}

func collectPairs(facts []ParsedFact) ([]string, []string) {
	stores, dates := map[string]bool{}, map[string]bool{}
	storeIDs, businessDates := make([]string, 0), make([]string, 0)
	for _, fact := range facts {
		if !stores[fact.StoreID] {
			stores[fact.StoreID] = true
			storeIDs = append(storeIDs, fact.StoreID)
		}
		if !dates[fact.BusinessDate] {
			dates[fact.BusinessDate] = true
			businessDates = append(businessDates, fact.BusinessDate)
		}
	}
	return storeIDs, businessDates
}

func columnOf(mapping Mapping, headers []string, field string) int {
	for index, header := range headers {
		if mapped, ok := mapping[header]; ok && mapped == field {
			return index
		}
	}
	return -1
}

func cellAt(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return row[index]
}

func valueAt(row []string, index int) string {
	return strings.TrimSpace(cellAt(row, index))
}

func headerFor(index int) string {
	if index < 0 {
		return ""
	}
	return fmt.Sprintf("col:%d", index+1)
}

func isUUID(value string) bool {
	return regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`).MatchString(value)
}

// normalizeKey folds a header or identifier for matching: lowercase,
// separator-free, full-width folded.
func normalizeKey(value string) string {
	var builder strings.Builder
	for _, char := range strings.TrimSpace(strings.ToLower(value)) {
		switch {
		case unicode.IsSpace(char), char == '_', char == '-', char == '.', char == '/', char == '·':
			continue
		case char == '（':
			builder.WriteRune('(')
		case char == '）':
			builder.WriteRune(')')
		case char == '：':
			builder.WriteRune(':')
		default:
			builder.WriteRune(char)
		}
	}
	return builder.String()
}

func parseBusinessDate(value string) (string, error) {
	for _, layout := range dateLayouts {
		if parsed, err := time.Parse(layout, strings.TrimSpace(value)); err == nil {
			return parsed.Format("2006-01-02"), nil
		}
	}
	return "", errors.New("unparseable business date")
}

func parseAmount(value string) (float64, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, errors.New("empty amount")
	}
	trimmed = strings.ReplaceAll(trimmed, ",", "")
	trimmed = strings.TrimSuffix(trimmed, "%")
	return strconv.ParseFloat(trimmed, 64)
}

var digitMask = regexp.MustCompile(`[0-9]`)

func maskDigits(value string) string {
	masked := digitMask.ReplaceAllString(value, "#")
	if len(masked) > 24 {
		return masked[:24]
	}
	return masked
}

// headerAliases is the deterministic suggestion table (normalized header →
// standard field). First matching column wins; the UI confirms.
var headerAliases = map[string]string{
	// store
	"门店": FieldStore, "门店编号": FieldStore, "门店代码": FieldStore, "门店id": FieldStore, "门店号": FieldStore,
	"店铺": FieldStore, "店铺编号": FieldStore, "店号": FieldStore, "store": FieldStore, "storeid": FieldStore, "storecode": FieldStore,
	// business_date
	"日期": FieldBusinessDate, "营业日期": FieldBusinessDate, "生意日期": FieldBusinessDate, "销售日期": FieldBusinessDate,
	"交易日期": FieldBusinessDate, "营业日": FieldBusinessDate, "date": FieldBusinessDate, "businessdate": FieldBusinessDate,
	// currency
	"币种": FieldCurrency, "货币": FieldCurrency, "币别": FieldCurrency, "currency": FieldCurrency,
	// revenue
	"营业额": FieldRevenue, "销售额": FieldRevenue, "营收": FieldRevenue, "门店销售额": FieldRevenue,
	"含税销售额": FieldRevenue, "不含税销售额": FieldRevenue, "sales": FieldRevenue, "revenue": FieldRevenue, "金额": FieldRevenue,
	// gross_profit
	"毛利": FieldGrossProfit, "毛利润": FieldGrossProfit, "毛利额": FieldGrossProfit, "grossprofit": FieldGrossProfit, "gp": FieldGrossProfit,
	// transactions
	"交易数": FieldTransactions, "交易笔数": FieldTransactions, "笔数": FieldTransactions, "单数": FieldTransactions,
	"客单数": FieldTransactions, "transactions": FieldTransactions, "transactioncount": FieldTransactions,
	// footfall
	"客流": FieldFootfall, "客流量": FieldFootfall, "进店人数": FieldFootfall, "人流": FieldFootfall, "footfall": FieldFootfall, "traffic": FieldFootfall,
	// area_sqm
	"面积": FieldAreaSqm, "建筑面积": FieldAreaSqm, "经营面积": FieldAreaSqm, "门店面积": FieldAreaSqm, "area": FieldAreaSqm, "areasqm": FieldAreaSqm,
	// labor_cost
	"人工成本": FieldLaborCost, "人工费用": FieldLaborCost, "工资": FieldLaborCost, "薪酬": FieldLaborCost,
	"人力成本": FieldLaborCost, "laborcost": FieldLaborCost, "labourcost": FieldLaborCost,
	// fixed_rent
	"固定租金": FieldFixedRent, "租金": FieldFixedRent, "基础租金": FieldFixedRent, "fixedrent": FieldFixedRent, "baserent": FieldFixedRent, "rent": FieldFixedRent,
	// variable_rent
	"变量租金": FieldVariableRent, "变动租金": FieldVariableRent, "抽成租金": FieldVariableRent,
	"联营扣点": FieldVariableRent, "提成租金": FieldVariableRent, "variablerent": FieldVariableRent, "turnoverrent": FieldVariableRent,
	// non_lease_cost
	"非租赁成本": FieldNonLeaseCost, "物业费": FieldNonLeaseCost, "物业管理费": FieldNonLeaseCost, "管理费": FieldNonLeaseCost,
	"推广费": FieldNonLeaseCost, "服务费": FieldNonLeaseCost, "nonleasecost": FieldNonLeaseCost, "cam": FieldNonLeaseCost,
	// other_controllable_cost
	"其他可控成本": FieldOtherControllableCost, "其他成本": FieldOtherControllableCost, "其他费用": FieldOtherControllableCost,
	"othercontrollablecost": FieldOtherControllableCost, "othercost": FieldOtherControllableCost,
}

// ─── IngestBatch 接缝（C3，架构重构任务书 2026-08-26）───────────────────────
//
// 此前编排散在 handlers/retail_ingest.go：解析 → 建议映射 → 门店归属 →
// 校验 → 批次行生命周期 → 提交 → 收尾，22 个调用点全在 handler 里。
// PreviewBatch 与 IngestBatch 把这条链收进引擎，handler 只剩 HTTP 参数
// 解析、端口接线和错误码到状态码的翻译。行为逐字节保持（锚：
// handlers 的 ingest golden + 真库集成测试）。

// BatchStorer owns the operating_fact_batches lifecycle rows.
type BatchStorer interface {
	CreateBatch(context.Context, *repository.OperatingFactBatch) (*repository.OperatingFactBatch, error)
	FinalizeBatch(context.Context, string, int, int, string, string, json.RawMessage) (*repository.OperatingFactBatch, error)
}

// WithBatchStore attaches the batch-lifecycle port.
func (s *Service) WithBatchStore(batches BatchStorer) *Service {
	s.batches = batches
	return s
}

// ImportFailureKind tells the transport layer which status/code family a
// failure maps to, so the mapping lives in one place instead of being
// re-derived from error strings.
type ImportFailureKind string

const (
	FailureParse       ImportFailureKind = "parse"         // template unreadable → 400 invalid_arguments
	FailureSystem      ImportFailureKind = "system"        // store/batch outage → 500 system_failure
	FailureEnvelope    ImportFailureKind = "envelope"      // provenance incomplete → 400 invalid_arguments
	FailureNoValidRows ImportFailureKind = "no_valid_rows" // every row rejected → 422 + report details
	FailureCommit      ImportFailureKind = "commit"        // coded commit failure → writeCodedFailure passthrough
)

// ImportError carries the exact message the previous handler wrote, plus the
// classification and (where the contract includes them) report and batch id.
type ImportError struct {
	Kind    ImportFailureKind
	Message string
	Err     error
	BatchID string
	Report  *ImportReport
}

func (e *ImportError) Error() string { return e.Message }

func (e *ImportError) Unwrap() error { return e.Err }

func failf(kind ImportFailureKind, err error) *ImportError {
	message := ""
	if err != nil {
		message = err.Error()
	}
	return &ImportError{Kind: kind, Message: message, Err: err}
}

// PreviewSpec is everything PreviewBatch needs. RawMapping is the client's
// corrected column mapping as posted; empty means "suggest one". It is decoded
// after the template parses, exactly where the previous handler did.
type PreviewSpec struct {
	LegalEntityID string
	Entity        access.EntityFilter
	SourceSystem  string
	File          []byte
	Format        Format
	RawMapping    string
}

// PreviewResult carries every field the preview response serializes.
type PreviewResult struct {
	Format              Format
	Headers             []string
	StandardFields      []string
	SuggestedMapping    Mapping
	SuggestionSource    string
	Mapping             Mapping
	RowsPreview         [][]string
	ColumnProfiles      []ColumnProfile
	ResolutionMatched   int
	ResolutionUnmatched []string
	Report              ValidationReport
}

// PreviewBatch runs the deterministic pipeline without writing: parse,
// suggest (unless the caller supplies a corrected mapping), resolve stores,
// validate rows, estimate overlap with existing facts.
func (s *Service) PreviewBatch(ctx context.Context, spec PreviewSpec) (*PreviewResult, error) {
	headers, rows, err := ParseTemplate(spec.File, spec.Format)
	if err != nil {
		return nil, failf(FailureParse, err)
	}
	profiles := ColumnProfiles(headers, rows)
	suggested, suggestionSource := s.SuggestMappingAssisted(ctx, headers, profiles)
	mapping := suggested
	confirmed, failure := confirmedMapping(spec.RawMapping)
	if failure != nil {
		return nil, failure
	}
	if confirmed != nil {
		mapping = confirmed
	}
	resolution, err := s.ResolveStores(ctx, spec.LegalEntityID, mapping, headers, rows)
	if err != nil {
		return nil, failf(FailureSystem, err)
	}
	report := Validate(headers, rows, mapping, resolution)
	coverage, err := s.EstimateOverlap(ctx, spec.LegalEntityID, spec.SourceSystem, report)
	if err != nil {
		return nil, failf(FailureSystem, err)
	}
	report.Coverage = coverage
	previewRows := rows
	if len(previewRows) > 8 {
		previewRows = previewRows[:8]
	}
	return &PreviewResult{
		Format: spec.Format, Headers: headers, StandardFields: AllFields,
		SuggestedMapping: suggested, SuggestionSource: suggestionSource, Mapping: mapping,
		RowsPreview: previewRows, ColumnProfiles: ColumnProfiles(headers, rows),
		ResolutionMatched: len(resolution.RawToStoreID), ResolutionUnmatched: resolution.Unmatched,
		Report: report,
	}, nil
}

// IngestSpec is one commit attempt. The engine owns batch creation, replay
// detection, envelope validation, chunked persistence and finalization.
type IngestSpec struct {
	LegalEntityID  string
	Entity         access.EntityFilter
	UserID         string
	Filename       string
	File           []byte
	Format         Format
	SourceSystem   string
	AsOf           time.Time
	IdempotencyKey string
	RawMapping     string
}

// IngestBatchResult is what the transport layer serializes. Envelope echoes
// the provenance triple verbatim — traceability fields are never rewritten.
type IngestBatchResult struct {
	Batch            *repository.OperatingFactBatch
	Report           *ImportReport
	SavedCount       int
	FailedCount      int
	IdempotentReplay bool
	Envelope         Envelope
}

// IngestBatch orchestrates validate → master data mapping → envelope →
// persist for one uploaded file, exactly as the handler used to, in order:
//
//  1. parse the controlled template;
//  2. map columns (confirmed human mapping wins over the rule/AI suggestion);
//  3. resolve raw store references against the entity-scoped master data;
//  4. create the batch row (idempotency key attached);
//  5. short-circuit replays of an already-finalized batch;
//  6. re-validate the envelope triple and persist in atomic chunks;
//  7. finalize the batch row with per-row errors attached.
func (s *Service) IngestBatch(ctx context.Context, spec IngestSpec) (*IngestBatchResult, error) {
	if s.batches == nil {
		return nil, &ImportError{Kind: FailureSystem, Message: "batch store is not wired"}
	}
	headers, rows, err := ParseTemplate(spec.File, spec.Format)
	if err != nil {
		return nil, failf(FailureParse, err)
	}
	mapping := SuggestMapping(headers, ColumnProfiles(headers, rows))
	confirmed, failure := confirmedMapping(spec.RawMapping)
	if failure != nil {
		return nil, failure
	}
	if confirmed != nil {
		mapping = confirmed
	}
	resolution, err := s.ResolveStores(ctx, spec.LegalEntityID, mapping, headers, rows)
	if err != nil {
		return nil, failf(FailureSystem, err)
	}
	var legalEntityPayload *string
	if scopedID, idErr := spec.Entity.LegalEntityID(); idErr == nil {
		legalEntityPayload = &scopedID
	}
	batch, err := s.batches.CreateBatch(ctx, &repository.OperatingFactBatch{
		LegalEntityID: legalEntityPayload, SourceSystem: spec.SourceSystem, SourceFile: spec.Filename,
		TotalRows: len(rows), AsOfAt: nowUTC(), CreatedBy: optionalString(spec.UserID),
		ReconciliationStatus: "unreconciled", IdempotencyKey: spec.IdempotencyKey,
		FactVersion: nowUTC().Format(time.RFC3339),
	})
	if err != nil {
		return nil, failf(FailureSystem, err)
	}
	if (batch.Status == "completed" || batch.Status == "failed") && batch.AcceptedRows+batch.RejectedRows > 0 {
		return &IngestBatchResult{
			Batch: batch, SavedCount: batch.AcceptedRows, FailedCount: batch.RejectedRows,
			IdempotentReplay: true,
		}, nil
	}
	envelope := Envelope{SourceSystem: spec.SourceSystem, ImportBatchID: batch.ID, AsOfAt: spec.AsOf}
	report, err := s.Commit(ctx, spec.LegalEntityID, headers, rows, mapping, resolution, envelope, spec.IdempotencyKey)
	if err != nil {
		switch {
		case errors.Is(err, ErrEnvelopeIncomplete):
			return nil, &ImportError{Kind: FailureEnvelope, Message: err.Error(), Err: err}
		case errors.Is(err, ErrNoValidRows):
			_, _ = s.batches.FinalizeBatch(ctx, batch.ID, 0, report.RejectedRows, "failed", "unreconciled", ErrorsJSON(report))
			return nil, &ImportError{Kind: FailureNoValidRows, Message: "every row failed validation", BatchID: batch.ID, Report: report}
		default:
			_, _ = s.batches.FinalizeBatch(ctx, batch.ID, report.AcceptedRows, report.RejectedRows, "failed", "unreconciled", ErrorsJSON(report))
			return nil, &ImportError{Kind: FailureCommit, Message: err.Error(), Err: err}
		}
	}
	batch, err = s.batches.FinalizeBatch(ctx, batch.ID, report.AcceptedRows, report.RejectedRows, "completed", "unreconciled", ErrorsJSON(report))
	if err != nil {
		return nil, failf(FailureSystem, err)
	}
	return &IngestBatchResult{
		Batch: batch, Report: report,
		SavedCount: report.AcceptedRows, FailedCount: report.RejectedRows,
		IdempotentReplay: report.ReplayDetected,
		Envelope:         envelope,
	}, nil
}

// ErrorsJSON renders the per-row report for the batch row's error column,
// as an empty array when there is nothing to record.
func ErrorsJSON(report *ImportReport) json.RawMessage {
	if report == nil || len(report.Errors) == 0 {
		return json.RawMessage(`[]`)
	}
	encoded, err := json.Marshal(report.Errors)
	if err != nil {
		return json.RawMessage(`[]`)
	}
	return encoded
}

// confirmedMapping decodes the human-confirmed {column: field} object. An
// empty raw means "no confirmation"; a malformed payload is a parse-class
// failure with the exact copy the handler always wrote.
func confirmedMapping(raw string) (Mapping, *ImportError) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	confirmed := Mapping{}
	if err := json.Unmarshal([]byte(raw), &confirmed); err != nil {
		return nil, &ImportError{Kind: FailureParse, Message: "mapping must be a JSON object of {column: field}"}
	}
	return confirmed, nil
}

func optionalString(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

// nowUTC is the engine's single wall clock, mirroring the handler helper the
// orchestration used before the seam existed.
func nowUTC() time.Time { return time.Now().UTC() }
