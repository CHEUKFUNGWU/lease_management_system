package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lease-management-system/core-service/internal/finmodel"
	"github.com/lease-management-system/core-service/internal/finmodel/opening"
	"github.com/lease-management-system/core-service/internal/finmodel/persist"
	"github.com/lease-management-system/core-service/internal/finmodel/template"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/audit"
	"github.com/lease-management-system/core-service/internal/services/currencytranslation"
)

// FinModelHandler is the /financial-model backend (PRD S2/S3/S5): templates
// with the S3-4 governance flow, definitions, assumption-driven runs with
// the publish gate, the opening three-gate validation, result/tie-out reads
// and the SM8 group view with its S5-2 translated second view.
type FinModelHandler struct {
	repo  *repository.FinModelRepository
	audit *audit.Logger
	fx    *repository.ExchangeRateRepository
	plans *repository.FPnAGovernanceRepository
	// cancels maps an async run id to its cancel func; entries disappear
	// when the goroutine exits.
	cancels sync.Map
}

// NewFinModelHandler builds the handler without an audit sink.
func NewFinModelHandler(repo *repository.FinModelRepository) *FinModelHandler {
	return &FinModelHandler{repo: repo}
}

// NewFinModelHandlerWithAudit builds the handler with the audit logger used
// for the governance transitions (关键动作: 复核 / 审批).
func NewFinModelHandlerWithAudit(repo *repository.FinModelRepository, auditLogger *audit.Logger) *FinModelHandler {
	return &FinModelHandler{repo: repo, audit: auditLogger}
}

// WithExchangeRates attaches the exchange-rate version reader the group
// view's translated second view translates through (S5-2).
func (h *FinModelHandler) WithExchangeRates(fx *repository.ExchangeRateRepository) *FinModelHandler {
	h.fx = fx
	return h
}

// WithPlanGovernance attaches the FP&A version repository the run-publish
// path writes its plan_version lineage into (S2-7).
func (h *FinModelHandler) WithPlanGovernance(plans *repository.FPnAGovernanceRepository) *FinModelHandler {
	h.plans = plans
	return h
}

// CreateTemplate saves a parsed template (illegal templates never persist).
func (h *FinModelHandler) CreateTemplate(c *gin.Context) {
	userID, ok := userID(c)
	if !ok {
		return
	}
	var def template.TemplateDef
	if err := decodeStrictJSON(c, &def); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	legalEntityID := middleware.GetTenantID(c)
	var entityPtr *string
	if legalEntityID != "" {
		entityPtr = &legalEntityID
	}
	if _, err := h.repo.SaveStatementTemplate(c.Request.Context(), def, entityPtr, &userID, uuid.NewString()); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"saved": true, "name": def.Name, "version": def.Version})
}

// ReviewTemplate is the reviewer hop of the S3-4 three-state flow.
// approved=true moves draft → review (or re-confirms review) and stamps the
// reviewer; approved=false returns a reviewed template to draft. The
// transition itself is guarded in the repository by the FROM status, so a
// concurrent writer cannot skip a state.
func (h *FinModelHandler) ReviewTemplate(c *gin.Context) {
	userID, ok := userID(c)
	if !ok {
		return
	}
	id := c.Param("id")
	if err := h.requireScopedTemplate(c, id); err != nil {
		return
	}
	var req struct {
		Approved bool   `json:"approved"`
		Reason   string `json:"reason,omitempty"`
	}
	if err := decodeStrictJSON(c, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.repo.ReviewStatementTemplate(c.Request.Context(), id, userID, req.Approved); err != nil {
		writeWorkflowMutationError(c, "review statement template", err)
		return
	}
	status, action := "in_review", "review"
	if !req.Approved {
		status, action = "returned_to_draft", "review_returned"
	}
	if h.audit != nil {
		_ = h.audit.Log(c.Request.Context(), "fin_statement_templates", id, action, nil, gin.H{"reason": req.Reason}, userID, c)
	}
	c.JSON(http.StatusOK, gin.H{"template_id": id, "status": status})
}

// ApproveTemplate is the approver hop: review → approved, stamped by the
// approver. The route chains RequireApprovalSeparation, so the approver can
// never be the creator or the reviewer of the same template. Once approved
// the template version is frozen — there is no edit path, only new versions.
func (h *FinModelHandler) ApproveTemplate(c *gin.Context) {
	userID, ok := userID(c)
	if !ok {
		return
	}
	id := c.Param("id")
	if err := h.requireScopedTemplate(c, id); err != nil {
		return
	}
	if err := h.repo.ApproveStatementTemplate(c.Request.Context(), id, userID); err != nil {
		writeWorkflowMutationError(c, "approve statement template", err)
		return
	}
	if h.audit != nil {
		_ = h.audit.Log(c.Request.Context(), "fin_statement_templates", id, "approve", nil, nil, userID, c)
	}
	c.JSON(http.StatusOK, gin.H{"template_id": id, "status": "approved"})
}

// CopyTemplate is the S3-4 copy action: the source's rows become a brand-new
// draft version. Copying under the same name continues the source lineage
// (version = source+1); a different name starts a fresh lineage at version
// 1. Either way the copy must pass review/approve like any creation.
func (h *FinModelHandler) CopyTemplate(c *gin.Context) {
	userID, ok := userID(c)
	if !ok {
		return
	}
	id := c.Param("id")
	if err := h.requireScopedTemplate(c, id); err != nil {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if c.Request.ContentLength > 0 {
		if err := decodeStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	source, err := h.repo.GetStatementTemplate(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return
	}
	name := strings.TrimSpace(req.Name)
	version := 1
	if name == "" || name == source.Name {
		name = source.Name
		version = source.Version + 1 // 同名复制继续同一谱系
	}
	newID := uuid.NewString()
	if err := h.repo.CopyStatementTemplate(c.Request.Context(), newID, name, version, &id, &userID); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	copied, err := h.repo.GetStatementTemplate(c.Request.Context(), newID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if h.audit != nil {
		_ = h.audit.Log(c.Request.Context(), "fin_statement_templates", id, "copy", nil, copied, userID, c)
	}
	c.JSON(http.StatusCreated, gin.H{"template": copied})
}

// DeleteTemplate is the S3-4 deletion rule made executable: only a draft
// that no model definition ever bound may be deleted — history and replay
// stay intact, the refusal names the reason.
func (h *FinModelHandler) DeleteTemplate(c *gin.Context) {
	userID, ok := userID(c)
	if !ok {
		return
	}
	id := c.Param("id")
	if err := h.requireScopedTemplate(c, id); err != nil {
		return
	}
	if err := h.repo.DeleteStatementTemplate(c.Request.Context(), id); err != nil {
		if errors.Is(err, repository.ErrStatementTemplateInUse) {
			c.JSON(http.StatusConflict, gin.H{"error": "模板已被模型定义引用，拒绝删除"})
			return
		}
		c.JSON(http.StatusConflict, gin.H{"error": "仅可删除从未被使用的草稿"})
		return
	}
	if h.audit != nil {
		_ = h.audit.Log(c.Request.Context(), "fin_statement_templates", id, "delete", nil, nil, userID, c)
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// ExportRun is the S2-9 export: GET /financial-model/runs/:id/export with
// an optional fold grain (month | quarter | year). The run's lines fold
// through the shared FoldMonthValues semantics (flows sum with缺口即缺失,
// balance-sheet stocks take the period end) and render as a live-formula
// workbook — subtotals are signed SUM expressions, the header carries
// data_classification (模拟标识), dataset and the version lines.
func (h *FinModelHandler) ExportRun(c *gin.Context) {
	id := c.Param("id")
	fold := finmodel.FoldKind(strings.TrimSpace(c.DefaultQuery("fold", "month")))
	if !finmodel.ValidFoldKind(string(fold)) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "fold must be month, quarter or year"})
		return
	}
	if h.repo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "repository unavailable"})
		return
	}
	run, err := h.repo.GetModelRun(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "run not found"})
		return
	}
	tenant := middleware.GetTenantID(c)
	if tenant != "" && run.LegalEntityID != tenant {
		c.JSON(http.StatusNotFound, gin.H{"error": "run not found"})
		return
	}
	def, err := h.repo.GetModelDefinition(c.Request.Context(), run.ModelDefinitionID)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "model definition not found"})
		return
	}
	tmpl, err := h.repo.LoadStatementTemplate(c.Request.Context(), def.TemplateID)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	lines, err := h.repo.ListRunLines(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	byRow := map[string]map[string]*float64{}
	months := map[string]bool{}
	for _, line := range lines {
		rowMap := byRow[line.RowKey]
		if rowMap == nil {
			rowMap = map[string]*float64{}
			byRow[line.RowKey] = rowMap
		}
		rowMap[line.Period] = line.Value
		months[line.Period] = true
	}
	periodList := make([]string, 0, len(months))
	for period := range months {
		periodList = append(periodList, period)
	}
	sortStrings(periodList)
	buckets := finmodel.FoldBuckets(periodList, fold)
	folded := finmodel.FoldMonthValues(byRow, buckets)

	rows := make([]modelExportRow, 0, len(tmpl.Rows))
	for _, row := range tmpl.Rows {
		rows = append(rows, modelExportRow{
			Key: row.Key, Label: row.Label, Kind: string(row.Kind), Basis: string(row.Basis),
			Children: row.Children, Subtracted: row.Subtract, Values: folded[row.Key],
		})
	}
	meta := ModelExportMeta{
		ModelName: def.Name, DataClassification: run.DataClassification,
		DatasetVersion: deref(run.DataVersion), AssumptionVersion: deref(run.AssumptionVersion),
		ExchangeRateVersion: deref(run.ExchangeRateVersion), MetricDefinitionVersion: deref(run.MetricDefinitionVersion),
		EngineVersion: "finmodel@" + itoaInt(run.ModelDefinitionVersion),
		FoldKind:      fold,
	}
	out, err := RenderModelRunXLSX(tmpl, rows, buckets, meta)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", "model-run-"+id+"-"+string(fold)+".xlsx"))
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", out)
}

func itoaInt(value int) string {
	return fmt.Sprintf("%d", value)
}

// PublishRun is the S2-7 publish action: one tie-out-passed run becomes a
// forecast plan_version with group-grain lines and prior_version_id lineage.
// Re-publishing the same run is idempotent (same version id, no second row).
func (h *FinModelHandler) PublishRun(c *gin.Context) {
	userID, ok := userID(c)
	if !ok {
		return
	}
	id := c.Param("id")
	if h.repo == nil || h.plans == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "publish repositories unavailable"})
		return
	}
	run, err := h.repo.GetModelRun(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "run not found"})
		return
	}
	tenant := middleware.GetTenantID(c)
	if tenant != "" && run.LegalEntityID != tenant {
		c.JSON(http.StatusNotFound, gin.H{"error": "run not found"})
		return
	}
	published, err := persist.NewPublishWriter(h.repo, h.plans).Publish(c.Request.Context(), id, &userID)
	if err != nil {
		if errors.Is(err, persist.ErrPublishGate) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if h.audit != nil {
		_ = h.audit.Log(c.Request.Context(), "fin_model_runs", id, "publish", nil, published, userID, c)
	}
	c.JSON(http.StatusCreated, gin.H{"published": published})
}

// requireScopedTemplate loads the target template and answers 404 unless it
// belongs to the caller's tenant (or the caller is a global admin): bottom
// line 1 — tenant A's reviewer cannot touch tenant B's template, whatever
// the id.
func (h *FinModelHandler) requireScopedTemplate(c *gin.Context, id string) error {
	if h.repo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "repository unavailable"})
		return errors.New("repository unavailable")
	}
	row, err := h.repo.GetStatementTemplate(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return err
	}
	tenant := middleware.GetTenantID(c)
	if tenant != "" && row.LegalEntityID != nil && *row.LegalEntityID != tenant {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return errors.New("template outside caller scope")
	}
	return nil
}

// ValidateOpening runs the SM4 three gates and returns the failures.
func (h *FinModelHandler) ValidateOpening(c *gin.Context) {
	var req struct {
		Balance  opening.OpeningBalance    `json:"balance"`
		LeaseRef []opening.ContractBalance `json:"lease_ref"`
		Engine   []opening.ContractBalance `json:"engine"`
		Policy   opening.MergePolicy       `json:"policy"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	failures := opening.Validate(opening.ValidateInput{Balance: req.Balance, LeaseRef: req.LeaseRef, Engine: req.Engine, Policy: req.Policy})
	c.JSON(http.StatusOK, gin.H{"failures": failures, "passed": len(failures) == 0})
}

// RunDefinitionRequest drives one run from an assumption set.
type RunDefinitionRequest struct {
	DefinitionID       string                     `json:"definition_id"`
	Title              string                     `json:"title,omitempty"`
	Assumptions        map[string]json.RawMessage `json:"assumptions"`
	DataClassification string                     `json:"data_classification,omitempty"`
	Versions           finmodel.VersionSet        `json:"versions"`
	IdempotencyKey     string                     `json:"idempotency_key,omitempty"`
	// Async dispatches the S2-5 async path: the run row is created queued
	// and the engine executes in the background; poll GET /runs/:id.
	Async bool `json:"async"`
}

// RunDefinition executes the pure engine on the definition template plus
// approved assumptions overlaid with the request's explicit values — nothing
// is recomputed outside SM2. Persist refuses failed tie-outs (publish gate).
func (h *FinModelHandler) RunDefinition(c *gin.Context) {
	userID, ok := userID(c)
	if !ok {
		return
	}
	var req RunDefinitionRequest
	if err := decodeStrictJSON(c, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defRow, err := h.repo.GetModelDefinition(c.Request.Context(), req.DefinitionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "model definition not found"})
		return
	}
	tmpl, err := h.repo.LoadStatementTemplate(c.Request.Context(), defRow.TemplateID)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	var policy finmodel.ModelPolicy
	if len(defRow.Policy) > 0 {
		_ = json.Unmarshal(defRow.Policy, &policy)
	}
	if policy.Version == "" {
		policy = finmodel.ModelPolicy{Version: "v1", InterestCashFlowPresentation: "financing"}
	}
	def := finmodel.ModelDef{
		Name: defRow.Name, LegalEntityID: defRow.LegalEntityID, Currency: req.Versions.Data,
		Template: tmpl, PeriodStart: periodStartOf(defRow.ActualCutoffPeriod),
		HistoricalMonths: 12, ForecastMonths: 24,
		ActualCutoffPeriod: deref(defRow.ActualCutoffPeriod),
		Policy:             policy,
	}
	if def.Currency == "" {
		def.Currency = "CNY"
	}
	inputs := finmodel.ModelInputs{
		Assumptions:        assumptionOverlay{repo: h.repo, legalEntityID: defRow.LegalEntityID, base: req.Assumptions, period: def.PeriodStart},
		Versions:           req.Versions,
		DataClassification: orDefault(req.DataClassification, "production"),
		// 事实与租赁端口的生产适配器随 GL/计量投影接线落地——缺失时诚实降级为缺口。
		Facts: nil, Lease: nil, Schedules: nil, Opening: nil,
	}

	if req.Async {
		h.dispatchAsyncRun(c, defRow, def, inputs, req, &userID)
		return
	}

	result, err := finmodel.Run(c.Request.Context(), def, inputs)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	payload := map[string]any{"run": result}
	if result.TieOutStatus != "failed" && h.repo != nil {
		idem := req.IdempotencyKey
		if idem == "" {
			idem = "run-" + uuid.NewString()
		}
		if err := persist.NewRunWriter(h.repo).Persist(c.Request.Context(), def, result, defRow.ID, idem, &userID); err != nil {
			if errors.Is(err, persist.ErrTieOutFailed) {
				c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error(), "run": result})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		payload["persisted"] = true
	}
	c.JSON(http.StatusOK, payload)
}

// asyncRunHook is nil in production; tests set it to hold a run between
// the engine and the persist so cancellation can race it deterministically.
var asyncRunHook func(runID string)

// dispatchAsyncRun is the S2-5 async path: the queued run row exists
// before the response (replays return the existing run), then the engine
// executes in the background and flips queued → running → completed /
// failed / cancelled. Status writes use a background context so a
// cancellation never races its own status update.
func (h *FinModelHandler) dispatchAsyncRun(c *gin.Context, defRow *repository.FinModelDefinition, def finmodel.ModelDef, inputs finmodel.ModelInputs, req RunDefinitionRequest, userID *string) {
	idem := req.IdempotencyKey
	if idem == "" {
		idem = "run-" + uuid.NewString()
	}
	if existing, err := h.repo.FindModelRunByIdempotency(c.Request.Context(), defRow.ID, idem); err == nil && existing != nil {
		c.JSON(http.StatusOK, gin.H{"run_id": existing.ID, "status": existing.Status, "replayed": true})
		return
	}
	snapshot, err := persist.Snapshot(def)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	runID := uuid.NewString()
	created := &repository.FinModelRun{
		ID: runID, LegalEntityID: defRow.LegalEntityID, ModelDefinitionID: defRow.ID,
		ModelDefinitionVersion: 1,
		DataVersion:            strPtr(req.Versions.Data), AssumptionVersion: strPtr(req.Versions.Assumption),
		ExchangeRateVersion: strPtr(req.Versions.ExchangeRate), MetricDefinitionVersion: strPtr(req.Versions.MetricDefinition),
		DataClassification: inputs.DataClassification,
		Status:             "queued", TieOutStatus: "pending",
		InputSnapshot: snapshot, IdempotencyKey: idem, CreatedBy: userID,
	}
	if err := h.repo.CreateModelRun(c.Request.Context(), created); err != nil {
		if errors.Is(err, repository.ErrFinModelRunReplay) {
			if existing, ferr := h.repo.FindModelRunByIdempotency(c.Request.Context(), defRow.ID, idem); ferr == nil && existing != nil {
				c.JSON(http.StatusOK, gin.H{"run_id": existing.ID, "status": existing.Status, "replayed": true})
				return
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	h.cancels.Store(runID, cancel)
	go func() {
		defer h.cancels.Delete(runID)
		defer cancel()
		// A panic inside the engine must land as a failed run with a reason —
		// never a zombie "running" row and never a crashed worker.
		defer func() {
			if recovered := recover(); recovered != nil {
				_ = h.repo.FailModelRun(context.Background(), runID, fmt.Sprintf("worker panic: %v", recovered))
			}
		}()
		_ = h.repo.UpdateModelRunStatus(context.Background(), runID, "running", "pending", nil)
		result, err := finmodel.Run(ctx, def, inputs)
		if err != nil {
			_ = h.repo.FailModelRun(context.Background(), runID, err.Error())
			return
		}
		if asyncRunHook != nil {
			asyncRunHook(runID)
		}
		select {
		case <-ctx.Done():
			_ = h.repo.CancelModelRun(context.Background(), runID)
			return
		default:
		}
		if err := persist.NewRunWriter(h.repo).PersistInto(ctx, runID, result); err != nil {
			_ = h.repo.FailModelRun(context.Background(), runID, err.Error())
			return
		}
	}()
	c.JSON(http.StatusAccepted, gin.H{"run_id": runID, "status": "queued"})
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// GetRun is the async progress read: GET /financial-model/runs/:id returns
// the run state and, once completed, the number of persisted lines.
func (h *FinModelHandler) GetRun(c *gin.Context) {
	run, ok := h.scopedRun(c)
	if !ok {
		return
	}
	lineCount := 0
	if run.Status == "completed" {
		if lines, err := h.repo.ListRunLines(c.Request.Context(), run.ID); err == nil {
			lineCount = len(lines)
		}
	}
	c.JSON(http.StatusOK, gin.H{"run": run, "line_count": lineCount})
}

// CancelRun stops a queued/running async run: the in-memory cancel func
// interrupts the worker, and the SQL guard handles runs whose worker lives
// elsewhere (or already finished).
func (h *FinModelHandler) CancelRun(c *gin.Context) {
	run, ok := h.scopedRun(c)
	if !ok {
		return
	}
	if cancel, found := h.cancels.Load(run.ID); found {
		cancel.(context.CancelFunc)()
	}
	if err := h.repo.CancelModelRun(c.Request.Context(), run.ID); err != nil {
		if errors.Is(err, repository.ErrInvalidWorkflowTransition) {
			c.JSON(http.StatusConflict, gin.H{"error": "run 已结束，无法取消"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if h.audit != nil {
		_ = h.audit.Log(c.Request.Context(), "fin_model_runs", run.ID, "cancel", nil, nil, userIDOf(c), c)
	}
	c.JSON(http.StatusOK, gin.H{"run_id": run.ID, "status": "cancelled"})
}

// scopedRun loads the target run and 404s unless it belongs to the caller's
// tenant (bottom line 1).
func (h *FinModelHandler) scopedRun(c *gin.Context) (*repository.FinModelRun, bool) {
	id := c.Param("id")
	if h.repo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "repository unavailable"})
		return nil, false
	}
	run, err := h.repo.GetModelRun(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "run not found"})
		return nil, false
	}
	tenant := middleware.GetTenantID(c)
	if tenant != "" && run.LegalEntityID != tenant {
		c.JSON(http.StatusNotFound, gin.H{"error": "run not found"})
		return nil, false
	}
	return run, true
}

func userIDOf(c *gin.Context) string {
	value, _ := c.Get("user_id")
	id, _ := value.(string)
	return id
}

// GroupRuns renders the SM8 aggregate: GET /financial-model/group with
// run_ids and an optional exchange_rate_version. The default response is
// the original-currency partition view; a translated second view is built
// only when exchange_rate_version is explicit, carries the rate version and
// its closing/average type, and omits every cross-currency total that the
// rates cannot back (T14 fail-closed).
func (h *FinModelHandler) GroupRuns(c *gin.Context) {
	runIDs := strings.Split(c.Query("run_ids"), ",")
	exchangeRateVersion := strings.TrimSpace(c.Query("exchange_rate_version"))
	rateType := strings.TrimSpace(c.Query("rate_type"))
	if len(runIDs) == 0 || (len(runIDs) == 1 && strings.TrimSpace(runIDs[0]) == "") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "run_ids is required"})
		return
	}
	tenant := middleware.GetTenantID(c)
	var members []finmodel.GroupRunInput
	authCurrencies := []string{}
	for _, id := range runIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if h.repo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "repository unavailable"})
			return
		}
		run, err := h.repo.GetModelRun(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "run not found: " + id})
			return
		}
		lines, err := h.repo.ListRunLines(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		values := map[string]*float64{}
		periodMap := map[string]bool{}
		for _, line := range lines {
			values[line.RowKey+"@"+line.Period] = line.Value
			periodMap[line.Period] = true
		}
		periods := make([]string, 0, len(periodMap))
		for period := range periodMap {
			periods = append(periods, period)
		}
		sortStrings(periods)
		authorized := run.LegalEntityID == tenant || tenant == ""
		member := finmodel.GroupRunInput{
			RunID: id, LegalEntityID: run.LegalEntityID,
			Authorized: authorized,
			Currency:   runCurrency(run), Periods: periods, Lines: values,
		}
		if authorized {
			authCurrencies = append(authCurrencies, member.Currency)
		}
		members = append(members, member)
	}

	targetCurrency := strings.ToUpper(strings.TrimSpace(c.Query("target_currency")))
	if targetCurrency == "" {
		targetCurrency = strings.ToUpper(strings.TrimSpace(c.Query("reporting_currency")))
	}
	if targetCurrency == "" && len(authCurrencies) > 0 {
		targetCurrency = authCurrencies[0]
	}

	// The translated second view (S5-2): translation happens BEFORE
	// Summarize so the aggregate is built exclusively from translated
	// values; an unresolvable pair degrades the whole translated view.
	exchangeRateType := ""
	if exchangeRateVersion != "" {
		if h.fx == nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "exchange_rate_version requested but the rate version reader is unavailable"})
			return
		}
		basis, err := currencytranslation.NewBasis(c.Request.Context(), exchangeRateVersion, &repoRateReader{repo: h.fx})
		if err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("failed to load exchange rate version %q: %v", exchangeRateVersion, err)})
			return
		}
		if rateType != "" && !strings.EqualFold(rateType, basis.Version().VersionType) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("rate_type %q conflicts with version type %q", rateType, basis.Version().VersionType)})
			return
		}
		exchangeRateType = basis.Version().VersionType
		for i := range members {
			if !members[i].Authorized {
				continue
			}
			members[i].ExchangeRateVersion = exchangeRateVersion
			rate, ok := basis.Rate(members[i].Currency, targetCurrency)
			if !ok {
				// 缺汇率：保留原币明细，但折算视图整体降级（fail-closed）。
				members[i].Note = "missing_exchange_rate"
				continue
			}
			translated := make(map[string]*float64, len(members[i].Lines))
			for key, v := range members[i].Lines {
				if v == nil {
					translated[key] = nil
					continue
				}
				tv := *v * rate
				translated[key] = &tv
			}
			members[i].TranslatedLines = translated
			members[i].TranslatedCurrency = targetCurrency
		}
	}

	summary, err := finmodel.Summarize(members, exchangeRateVersion)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	summary.ExchangeRateType = exchangeRateType
	c.JSON(http.StatusOK, gin.H{"group": summary})
}

// runCurrency derives a run's currency from its input snapshot (the engine
// writes it); when the snapshot is absent or unreadable the CNY default
// keeps old runs legible instead of making the group view unusable.
func runCurrency(run *repository.FinModelRun) string {
	var snap struct {
		Currency string `json:"currency"`
	}
	if len(run.InputSnapshot) > 0 {
		if err := json.Unmarshal(run.InputSnapshot, &snap); err == nil {
			if cur := strings.ToUpper(strings.TrimSpace(snap.Currency)); cur != "" {
				return cur
			}
		}
	}
	return "CNY"
}

func (h *FinModelHandler) ListDefinitions(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"definitions": []any{}}) // 列表 UI 逐步填充；创建与运行已是真实路径
}

// assumptionOverlay reads approved values first, then overlays the request's
// explicit assumption set — drafts never appear because the reader queries
// status='approved' only.
type assumptionOverlay struct {
	repo          *repository.FinModelRepository
	legalEntityID string
	base          map[string]json.RawMessage
	period        string
}

func (o assumptionOverlay) Value(ctx context.Context, _, key, _ string) (json.RawMessage, error) {
	if raw, ok := o.base[key]; ok {
		return raw, nil
	}
	if o.repo == nil {
		return nil, nil
	}
	values, err := o.repo.LatestApprovedAssumptions(ctx, o.legalEntityID, []string{key}, o.period)
	if err != nil {
		return nil, err
	}
	return values[key], nil
}

func userID(c *gin.Context) (string, bool) {
	value, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user context"})
		return "", false
	}
	id, _ := value.(string)
	return id, true
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func orDefault(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

func sortStrings(values []string) { sort.Strings(values) }

func periodStartOf(cutoff *string) string {
	if cutoff == nil {
		return time.Now().AddDate(-1, 0, 0).Format("2006-01")
	}
	t, err := time.Parse("2006-01", *cutoff)
	if err != nil {
		return time.Now().AddDate(-1, 0, 0).Format("2006-01")
	}
	return t.AddDate(-11, 0, 0).Format("2006-01")
}
