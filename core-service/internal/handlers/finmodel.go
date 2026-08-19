package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
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
)

// FinModelHandler is the /financial-model backend (PRD S2/S3): templates
// with the S3-4 governance flow, definitions, assumption-driven runs with
// the publish gate, the opening three-gate validation and result/tie-out
// reads.
type FinModelHandler struct {
	repo  *repository.FinModelRepository
	audit *audit.Logger
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

// GroupRuns is GET /financial-model/group?run_ids=a,b&exchange_rate_version=. It
// renders the SM8 aggregate: explicit unauthorized members, currency
// discipline and ties-out with the member details.
func (h *FinModelHandler) GroupRuns(c *gin.Context) {
	runIDs := strings.Split(c.Query("run_ids"), ",")
	exchangeRateVersion := strings.TrimSpace(c.Query("exchange_rate_version"))
	if len(runIDs) == 0 || (len(runIDs) == 1 && strings.TrimSpace(runIDs[0]) == "") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "run_ids is required"})
		return
	}
	tenant := middleware.GetTenantID(c)
	var members []finmodel.GroupRunInput
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
		members = append(members, finmodel.GroupRunInput{
			RunID: id, LegalEntityID: run.LegalEntityID,
			Authorized: run.LegalEntityID == tenant || tenant == "",
			Currency:   "CNY", Periods: periods, Lines: values,
		})
	}
	summary, err := finmodel.Summarize(members, exchangeRateVersion)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"group": summary})
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
