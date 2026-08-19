package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lease-management-system/core-service/internal/finmodel/template"
)

func uuidNew() string { return uuid.NewString() }

// FinStatementTemplate is one versioned, frozen statement template row. Rows
// is the template def's rows JSONB payload.
type FinStatementTemplate struct {
	ID            string          `json:"id"`
	LegalEntityID *string         `json:"legal_entity_id,omitempty"`
	Name          string          `json:"name"`
	Version       int             `json:"version"`
	Status        string          `json:"status"`
	Rows          json.RawMessage `json:"rows"`
	CreatedBy     *string         `json:"created_by,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	ApprovedBy    *string         `json:"approved_by,omitempty"`
	ApprovedAt    *time.Time      `json:"approved_at,omitempty"`
}

// FinModelDefinition is one legal-entity-scoped model definition bound to a
// template version with policy parameters (interest method, tax policy,
// interest cash-flow presentation) and source bindings.
type FinModelDefinition struct {
	ID                 string          `json:"id"`
	LegalEntityID      string          `json:"legal_entity_id"`
	Name               string          `json:"name"`
	Version            int             `json:"version"`
	TemplateID         string          `json:"template_id"`
	ActualCutoffPeriod *string         `json:"actual_cutoff_period,omitempty"`
	Policy             json.RawMessage `json:"policy"`
	SourceBindings     json.RawMessage `json:"source_bindings"`
	Status             string          `json:"status"`
	CreatedBy          *string         `json:"created_by,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

// FinModelRun is one model run: the four version lines + data classification
// + idempotency key + tie-out summary.
type FinModelRun struct {
	ID                      string          `json:"id"`
	LegalEntityID           string          `json:"legal_entity_id"`
	ModelDefinitionID       string          `json:"model_definition_id"`
	ModelDefinitionVersion  int             `json:"model_definition_version"`
	DataVersion             *string         `json:"data_version,omitempty"`
	AssumptionVersion       *string         `json:"assumption_version,omitempty"`
	ExchangeRateVersion     *string         `json:"exchange_rate_version,omitempty"`
	MetricDefinitionVersion *string         `json:"metric_definition_version,omitempty"`
	DataClassification      string          `json:"data_classification"`
	Status                  string          `json:"status"`
	TieOutStatus            string          `json:"tie_out_status"`
	InputSnapshot           json.RawMessage `json:"input_snapshot"`
	IdempotencyKey          string          `json:"idempotency_key"`
	CreatedBy               *string         `json:"created_by,omitempty"`
	CreatedAt               time.Time       `json:"created_at"`
	CompletedAt             *time.Time      `json:"completed_at,omitempty"`
}

// FinModelRunLine is one (row × period) result cell with provenance. Value is
// nil when the cell is missing — never zero-filled.
type FinModelRunLine struct {
	RunID      string          `json:"run_id"`
	RowKey     string          `json:"row_key"`
	Period     string          `json:"period"`
	Value      *float64        `json:"value"`
	Currency   *string         `json:"currency,omitempty"`
	Provenance json.RawMessage `json:"provenance"`
}

// FinModelTieOut is one executed tie-out check.
type FinModelTieOut struct {
	RunID     string   `json:"run_id"`
	CheckCode string   `json:"check_code"`
	Period    string   `json:"period"`
	Expected  *float64 `json:"expected"`
	Actual    *float64 `json:"actual"`
	Diff      *float64 `json:"diff"`
	Status    string   `json:"status"`
}

// ErrFinModelRunReplay is returned when the same (definition, idempotency
// key) run already exists — replays must not create a second record.
var ErrFinModelRunReplay = errors.New("finmodel run replay: idempotency key already used")

// FinModelRepository is the persistence seam for the five model tables.
type FinModelRepository struct{ db DBTX }

// NewFinModelRepository builds the repository over the pool.
func NewFinModelRepository(db DBTX) *FinModelRepository { return &FinModelRepository{db: db} }

const finTemplateCols = "id, legal_entity_id, name, version, status, rows, created_by, created_at, approved_by, approved_at"

func (r *FinModelRepository) CreateStatementTemplate(ctx context.Context, t *FinStatementTemplate) error {
	_, err := r.db.Exec(ctx, `INSERT INTO fin_statement_templates
		(id, legal_entity_id, name, version, status, rows, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		t.ID, t.LegalEntityID, t.Name, t.Version, t.Status, t.Rows, t.CreatedBy)
	return err
}

func (r *FinModelRepository) GetStatementTemplate(ctx context.Context, id string) (*FinStatementTemplate, error) {
	var t FinStatementTemplate
	err := r.db.QueryRow(ctx, `SELECT `+finTemplateCols+` FROM fin_statement_templates WHERE id=$1`, id).
		Scan(&t.ID, &t.LegalEntityID, &t.Name, &t.Version, &t.Status, &t.Rows, &t.CreatedBy, &t.CreatedAt, &t.ApprovedBy, &t.ApprovedAt)
	return &t, err
}

func (r *FinModelRepository) FindStatementTemplate(ctx context.Context, legalEntityID *string, name string, version int) (*FinStatementTemplate, error) {
	var t FinStatementTemplate
	err := r.db.QueryRow(ctx, `SELECT `+finTemplateCols+` FROM fin_statement_templates
		WHERE name=$1 AND version=$2 AND (legal_entity_id=$3 OR (legal_entity_id IS NULL AND $3::uuid IS NULL))
		ORDER BY created_at DESC LIMIT 1`, name, version, legalEntityID).
		Scan(&t.ID, &t.LegalEntityID, &t.Name, &t.Version, &t.Status, &t.Rows, &t.CreatedBy, &t.CreatedAt, &t.ApprovedBy, &t.ApprovedAt)
	return &t, err
}

func (r *FinModelRepository) UpdateStatementTemplateStatus(ctx context.Context, id, status string, approver *string) error {
	_, err := r.db.Exec(ctx, `UPDATE fin_statement_templates SET status=$2, approved_by=$3, approved_at=NOW() WHERE id=$1`, id, status, approver)
	return err
}

func (r *FinModelRepository) CreateModelDefinition(ctx context.Context, d *FinModelDefinition) error {
	_, err := r.db.Exec(ctx, `INSERT INTO fin_model_definitions
		(id, legal_entity_id, name, version, template_id, actual_cutoff_period, policy, source_bindings, status, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		d.ID, d.LegalEntityID, d.Name, d.Version, d.TemplateID, d.ActualCutoffPeriod, d.Policy, d.SourceBindings, d.Status, d.CreatedBy)
	return err
}

func (r *FinModelRepository) GetModelDefinition(ctx context.Context, id string) (*FinModelDefinition, error) {
	var d FinModelDefinition
	err := r.db.QueryRow(ctx, `SELECT id, legal_entity_id, name, version, template_id, actual_cutoff_period, policy, source_bindings, status, created_by, created_at, updated_at
		FROM fin_model_definitions WHERE id=$1`, id).
		Scan(&d.ID, &d.LegalEntityID, &d.Name, &d.Version, &d.TemplateID, &d.ActualCutoffPeriod, &d.Policy, &d.SourceBindings, &d.Status, &d.CreatedBy, &d.CreatedAt, &d.UpdatedAt)
	return &d, err
}

func (r *FinModelRepository) CreateModelRun(ctx context.Context, run *FinModelRun) error {
	_, err := r.db.Exec(ctx, `INSERT INTO fin_model_runs
		(id, legal_entity_id, model_definition_id, model_definition_version, data_version, assumption_version, exchange_rate_version, metric_definition_version, data_classification, status, tie_out_status, input_snapshot, idempotency_key, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		run.ID, run.LegalEntityID, run.ModelDefinitionID, run.ModelDefinitionVersion,
		run.DataVersion, run.AssumptionVersion, run.ExchangeRateVersion, run.MetricDefinitionVersion,
		run.DataClassification, run.Status, run.TieOutStatus, run.InputSnapshot, run.IdempotencyKey, run.CreatedBy)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation on (model_definition_id, idempotency_key)
			return ErrFinModelRunReplay
		}
		return err
	}
	return nil
}

func (r *FinModelRepository) GetModelRun(ctx context.Context, id string) (*FinModelRun, error) {
	var run FinModelRun
	err := r.db.QueryRow(ctx, `SELECT id, legal_entity_id, model_definition_id, model_definition_version, data_version, assumption_version, exchange_rate_version, metric_definition_version, data_classification, status, tie_out_status, input_snapshot, idempotency_key, created_by, created_at, completed_at
		FROM fin_model_runs WHERE id=$1`, id).
		Scan(&run.ID, &run.LegalEntityID, &run.ModelDefinitionID, &run.ModelDefinitionVersion,
			&run.DataVersion, &run.AssumptionVersion, &run.ExchangeRateVersion, &run.MetricDefinitionVersion,
			&run.DataClassification, &run.Status, &run.TieOutStatus, &run.InputSnapshot, &run.IdempotencyKey,
			&run.CreatedBy, &run.CreatedAt, &run.CompletedAt)
	return &run, err
}

func (r *FinModelRepository) UpdateModelRunStatus(ctx context.Context, id, status, tieOutStatus string, completedAt *time.Time) error {
	_, err := r.db.Exec(ctx, `UPDATE fin_model_runs SET status=$2, tie_out_status=$3, completed_at=$4 WHERE id=$1`,
		id, status, tieOutStatus, completedAt)
	return err
}

// InsertRunLines upserts result cells. ON CONFLICT DO NOTHING keeps replays
// idempotent: a replayed run never duplicates lines.
func (r *FinModelRepository) InsertRunLines(ctx context.Context, lines []FinModelRunLine) error {
	if len(lines) == 0 {
		return nil
	}
	values := make([]any, 0, len(lines)*6)
	rows := ""
	for i, line := range lines {
		if i > 0 {
			rows += ","
		}
		rows += fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d)", i*6+1, i*6+2, i*6+3, i*6+4, i*6+5, i*6+6)
		values = append(values, line.RunID, line.RowKey, line.Period, line.Value, line.Currency, line.Provenance)
	}
	_, err := r.db.Exec(ctx, `INSERT INTO fin_model_run_lines (run_id, row_key, period, value, currency, provenance)
		VALUES `+rows+` ON CONFLICT (run_id, row_key, period) DO NOTHING`, values...)
	if err != nil {
		return fmt.Errorf("insert fin_model_run_lines: %w", err)
	}
	return nil
}

func (r *FinModelRepository) InsertTieOuts(ctx context.Context, outs []FinModelTieOut) error {
	if len(outs) == 0 {
		return nil
	}
	values := make([]any, 0, len(outs)*7)
	rows := ""
	for i, out := range outs {
		if i > 0 {
			rows += ","
		}
		rows += fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d,$%d)", i*7+1, i*7+2, i*7+3, i*7+4, i*7+5, i*7+6, i*7+7)
		values = append(values, out.RunID, out.CheckCode, out.Period, out.Expected, out.Actual, out.Diff, out.Status)
	}
	_, err := r.db.Exec(ctx, `INSERT INTO fin_model_tie_outs (run_id, check_code, period, expected, actual, diff, status)
		VALUES `+rows+` ON CONFLICT (run_id, check_code, period) DO NOTHING`, values...)
	if err != nil {
		return fmt.Errorf("insert fin_model_tie_outs: %w", err)
	}
	return nil
}

func (r *FinModelRepository) ListRunLines(ctx context.Context, runID string) ([]*FinModelRunLine, error) {
	rows, err := r.db.Query(ctx, `SELECT run_id, row_key, period, value, currency, provenance
		FROM fin_model_run_lines WHERE run_id=$1 ORDER BY period, row_key`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*FinModelRunLine
	for rows.Next() {
		var line FinModelRunLine
		if err := rows.Scan(&line.RunID, &line.RowKey, &line.Period, &line.Value, &line.Currency, &line.Provenance); err != nil {
			return nil, err
		}
		out = append(out, &line)
	}
	return out, rows.Err()
}

func (r *FinModelRepository) ListTieOuts(ctx context.Context, runID string) ([]*FinModelTieOut, error) {
	rows, err := r.db.Query(ctx, `SELECT run_id, check_code, period, expected, actual, diff, status
		FROM fin_model_tie_outs WHERE run_id=$1 ORDER BY period, check_code`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*FinModelTieOut
	for rows.Next() {
		var t FinModelTieOut
		if err := rows.Scan(&t.RunID, &t.CheckCode, &t.Period, &t.Expected, &t.Actual, &t.Diff, &t.Status); err != nil {
			return nil, err
		}
		out = append(out, &t)
	}
	return out, rows.Err()
}

// SaveAssumptionDrafts persists AI suggestion drafts (status=draft,
// source=ai_suggestion), evidence and derived confidence included. The
// engine's AssumptionReader reads only approved rows (LatestApproved...),
// so these drafts can never leak into a formal run until a human approves.
func (r *FinModelRepository) SaveAssumptionDrafts(ctx context.Context, legalEntityID string, rows []AssumptionDraftRow, idempotencyKey string) ([]string, error) {
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		id := uuidNew()
		_, err := r.db.Exec(ctx, `INSERT INTO fpna_assumption_versions
			(id, legal_entity_id, assumption_key, category, value, unit, source, owner_name, effective_from, effective_to, version, status, evidence, confidence)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
			id, legalEntityID, row.Key, row.Category, row.Value, row.Unit, row.Source, row.Owner,
			row.EffectiveFrom, nil, 1, "draft", row.Evidence, row.Confidence)
		if err != nil {
			return nil, fmt.Errorf("save assumption drafts(%s): %w", idempotencyKey, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// AssumptionDraftRow is one draft insert.
type AssumptionDraftRow struct {
	Key           string
	Category      string
	Value         json.RawMessage
	Unit          string
	Source        string
	Owner         string
	EffectiveFrom time.Time
	Evidence      json.RawMessage
	Confidence    *float64
}

// LatestApprovedAssumptions reads the newest approved value per key — the
// production AssumptionReader for the model engine reads ONLY this path, so
// a status other than approved has no route into a run.
func (r *FinModelRepository) LatestApprovedAssumptions(ctx context.Context, legalEntityID string, keys []string, period string) (map[string]json.RawMessage, error) {
	out := map[string]json.RawMessage{}
	rows, err := r.db.Query(ctx, `SELECT DISTINCT ON (assumption_key) assumption_key, value
		FROM fpna_assumption_versions
		WHERE legal_entity_id=$1 AND assumption_key = ANY($2::varchar[]) AND status='approved'
		  AND effective_from <= $3::date AND (effective_to IS NULL OR effective_to >= $3::date)
		ORDER BY assumption_key, version DESC, effective_from DESC`, legalEntityID, keys, period+"-01")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var value json.RawMessage
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		out[key] = value
	}
	return out, rows.Err()
}

// — TemplateStore Postgres adapter (SM1 seam) —

// SaveStatementTemplate parses-then-persists: an illegal template never
// reaches Postgres, and a frozen version is never overwritten.
func (r *FinModelRepository) SaveStatementTemplate(ctx context.Context, def template.TemplateDef, legalEntityID *string, createdBy *string, id string) (*template.Template, error) {
	tmpl, err := template.Parse(def)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(def)
	if err != nil {
		return nil, err
	}
	if err := r.CreateStatementTemplate(ctx, &FinStatementTemplate{
		ID: id, LegalEntityID: legalEntityID, Name: tmpl.Name, Version: tmpl.Major,
		Status: "draft", Rows: raw, CreatedBy: createdBy,
	}); err != nil {
		return nil, fmt.Errorf("save statement template: %w", err)
	}
	return tmpl, nil
}

// LoadStatementTemplate reads a persisted template back through Parse — the
// read path re-validates, so hand-edited JSONB can never smuggle in an
// illegal structure.
func (r *FinModelRepository) LoadStatementTemplate(ctx context.Context, id string) (*template.Template, error) {
	row, err := r.GetStatementTemplate(ctx, id)
	if err != nil {
		return nil, err
	}
	var def template.TemplateDef
	if err := json.Unmarshal(row.Rows, &def); err != nil {
		return nil, fmt.Errorf("statement template %s rows corrupted: %w", id, err)
	}
	return template.Parse(def)
}
