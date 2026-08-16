package handlers

import (
	"context"
	"crypto/sha256"
	"io"
	"encoding/hex"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/errcontract"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/controlledintake"
)

// TrialBalanceHandler imports and lists versioned, content-identified GL
// trial balances (ADR-0009, PRD P5-50). Functional currency is mandatory —
// reconciliation conclusions rest on it — and the content identity is the
// (entity, source, period, sha256) tuple: re-importing the same extract
// replays the same version.
type TrialBalanceHandler struct {
	repo trialBalanceStore
}

type trialBalanceStore interface {
	CreateTrialBalanceVersion(ctx context.Context, item *repository.TrialBalanceVersion) (*repository.TrialBalanceVersion, error)
	GetTrialBalanceVersionByContent(ctx context.Context, entity access.EntityFilter, sourceSystem, period, sha256 string) (*repository.TrialBalanceVersion, error)
	CreateTrialBalanceLine(ctx context.Context, item *repository.TrialBalanceLine) (*repository.TrialBalanceLine, error)
	ListTrialBalanceVersions(ctx context.Context, entity access.EntityFilter, period string) ([]*repository.TrialBalanceVersion, error)
	DeleteTrialBalanceVersion(ctx context.Context, id string) error
}

func NewTrialBalanceHandler(repo trialBalanceStore) *TrialBalanceHandler { return &TrialBalanceHandler{repo: repo} }

var tbCurrencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

func (h *TrialBalanceHandler) Import(c *gin.Context) {
	legalEntityID := strings.TrimSpace(middleware.GetTenantID(c))
	if legalEntityID == "" {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "legal_entity_id is required", nil)
		return
	}
	entity, ok := tenantEntity(c)
	if !ok {
		writeCodedError(c, http.StatusForbidden, errcontract.CodePermissionDenied, "legal entity scope is required", nil)
		return
	}
	name := strings.TrimSpace(c.PostForm("name"))
	sourceSystem := strings.TrimSpace(c.PostForm("source_system"))
	period := strings.TrimSpace(c.PostForm("period"))
	functionalCurrency := strings.ToUpper(strings.TrimSpace(c.PostForm("functional_currency")))
	if name == "" || sourceSystem == "" || !planPeriodPattern.MatchString(period) || !tbCurrencyPattern.MatchString(functionalCurrency) {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "name, source_system, period (YYYY-MM) and functional_currency (3-letter ISO) are required", nil)
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "file is required", nil)
		return
	}
	if fileHeader.Size > maxRetailIngestFileSize {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "file exceeds the 10MB import limit", nil)
		return
	}
	opened, err := fileHeader.Open()
	if err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "file cannot be opened", nil)
		return
	}
	defer opened.Close()
	data, err := io.ReadAll(opened)
	if err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "file cannot be read", nil)
		return
	}
	headers, parsedRows, err := controlledintake.Parse(controlledintake.Source{Filename: fileHeader.Filename, Data: data})
	if err != nil {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, err.Error(), nil)
		return
	}
	header := map[string]int{}
	for index, value := range headers {
		header[strings.ToLower(strings.TrimSpace(value))] = index
	}
	for _, required := range []string{"account_code", "debit", "credit"} {
		if _, exists := header[required]; !exists {
			writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "template requires account_code, debit, credit columns (account_name optional)", nil)
			return
		}
	}
	rows := parsedRows
	// Content identity: canonical normalized rows hashed — the same extract
	// in any encoding yields the same hash and replays the same version.
	canonical := make([]string, 0, len(rows)-1)
	var totalDebit, totalCredit float64
	lineErrors := controlledintake.NewReporter()
	parsed := make([]repository.TrialBalanceLine, 0)
	seenAccounts := map[string]bool{}
	for index, row := range rows {
		get := func(key string) string {
			position, ok := header[key]
			if !ok || position >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[position])
		}
		code := get("account_code")
		if code == "" {
			lineErrors.Add(index+2, "missing_account_code", "account_code is required")
			continue
		}
		// P1-2: a repeated account code within one file is a data error, not
		// a silent drop — otherwise the stored lines would no longer add up
		// to the reported totals.
		if seenAccounts[code] {
			lineErrors.Add(index+2, "duplicate_account_code", "account_code "+code+" appears more than once")
			continue
		}
		seenAccounts[code] = true
		debit, debitErr := strconv.ParseFloat(strings.ReplaceAll(get("debit"), ",", ""), 64)
		credit, creditErr := strconv.ParseFloat(strings.ReplaceAll(get("credit"), ",", ""), 64)
		if debitErr != nil || creditErr != nil || debit < 0 || credit < 0 {
			lineErrors.Add(index+2, "bad_amount", "debit/credit must be non-negative numbers")
			continue
		}
		canonical = append(canonical, code+"|"+strconv.FormatFloat(debit, 'f', 2, 64)+"|"+strconv.FormatFloat(credit, 'f', 2, 64))
		totalDebit += debit
		totalCredit += credit
		parsed = append(parsed, repository.TrialBalanceLine{AccountCode: code, AccountName: get("account_name"), Debit: debit, Credit: credit})
	}
	if len(parsed) == 0 {
		writeCodedError(c, http.StatusUnprocessableEntity, errcontract.CodeInvalidArguments, "every row failed validation", gin.H{"errors": lineErrors.Errors()})
		return
	}
	content := strings.Join(canonical, "\n")
	digest := sha256.Sum256([]byte(content))
	contentSHA256 := hex.EncodeToString(digest[:])
	version := &repository.TrialBalanceVersion{
		LegalEntityID: &legalEntityID, Name: name, SourceSystem: sourceSystem, Period: period,
		FunctionalCurrency: functionalCurrency, ContentSHA256: contentSHA256,
		TotalDebit: totalDebit, TotalCredit: totalCredit, CreatedBy: optionalString(userIDFromContext(c)),
	}
	created, err := h.repo.CreateTrialBalanceVersion(c.Request.Context(), version)
	if errors.Is(err, repository.ErrTrialBalanceVersionReplay) {
		existing, resolveErr := h.repo.GetTrialBalanceVersionByContent(c.Request.Context(), entity, sourceSystem, period, contentSHA256)
		if resolveErr != nil {
			writeSystemFailure(c, http.StatusInternalServerError, resolveErr)
			return
		}
		c.JSON(http.StatusOK, gin.H{"basis": "Working", "version": existing, "accepted_rows": 0, "rejected_rows": 0, "idempotent_replay": true, "errors": []controlledintake.RowError{}})
		return
	}
	if err != nil {
		writeCodedFailure(c, http.StatusUnprocessableEntity, err, nil)
		return
	}
	for index := range parsed {
		parsed[index].TrialBalanceVersionID = created.ID
		if _, err := h.repo.CreateTrialBalanceLine(c.Request.Context(), &parsed[index]); err != nil {
			// P1-2: compensate the partial version (cascade removes lines).
			_ = h.repo.DeleteTrialBalanceVersion(c.Request.Context(), created.ID)
			writeCodedFailure(c, http.StatusUnprocessableEntity, err, nil)
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"basis": "Working", "version": created, "accepted_rows": len(parsed),
		"rejected_rows": lineErrors.Count(), "idempotent_replay": false, "errors": lineErrors.Errors(),
		"balanced": totalDebit == totalCredit,
	})
}

func (h *TrialBalanceHandler) List(c *gin.Context) {
	entity, ok := tenantEntity(c)
	if !ok {
		writeCodedError(c, http.StatusForbidden, errcontract.CodePermissionDenied, "legal entity scope is required", nil)
		return
	}
	period := strings.TrimSpace(c.Query("period"))
	if period != "" && !planPeriodPattern.MatchString(period) {
		writeCodedError(c, http.StatusBadRequest, errcontract.CodeInvalidArguments, "period must be YYYY-MM", nil)
		return
	}
	versions, err := h.repo.ListTrialBalanceVersions(c.Request.Context(), entity, period)
	if err != nil {
		writeSystemFailure(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"basis": "Working", "total": len(versions), "data": versions})
}
