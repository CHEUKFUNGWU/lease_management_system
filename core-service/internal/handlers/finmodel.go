package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
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
)

// FinModelHandler is the /financial-model backend (PRD S2/S3): templates,
// definitions, assumption-driven runs with the publish gate, the opening
// three-gate validation and result/tie-out reads.
type FinModelHandler struct {
	repo *repository.FinModelRepository
}

// NewFinModelHandler builds the handler.
func NewFinModelHandler(repo *repository.FinModelRepository) *FinModelHandler {
	return &FinModelHandler{repo: repo}
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
