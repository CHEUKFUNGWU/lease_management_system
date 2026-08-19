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
	ReviewedBy    *string         `json:"reviewed_by,omitempty"`
	ReviewedAt    *time.Time      `json:"reviewed_at,omitempty"`
	ApprovedBy    *string         `json:"approved_by,omitempty"`
	ApprovedAt    *time.Time      `json:"approved_at,omitempty"`
	// CopiedFrom anchors the S3-4 copy lineage: the template this version
	// was copied from, nil for original creations.
	CopiedFrom *string `json:"copied_from,omitempty"`
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
	// FailureReason holds why an async run failed — honest progress, not a
	// bare status flip (S2-5).
	FailureReason *string `json:"failure_reason,omitempty"`
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

const finTemplateCols = "id, legal_entity_id, name, version, status, rows, created_by, created_at, reviewed_by, reviewed_at, approved_by, approved_at, copied_from"

// rowScanner is the Scan surface shared by pgx.Row and pgx.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanStatementTemplate(s rowScanner) (*FinStatementTemplate, error) {
	var t FinStatementTemplate
	err := s.Scan(&t.ID, &t.LegalEntityID, &t.Name, &t.Version, &t.Status, &t.Rows,
		&t.CreatedBy, &t.CreatedAt, &t.ReviewedBy, &t.ReviewedAt, &t.ApprovedBy, &t.ApprovedAt, &t.CopiedFrom)
	return &t, err
}

func (r *FinModelRepository) CreateStatementTemplate(ctx context.Context, t *FinStatementTemplate) error {
	_, err := r.db.Exec(ctx, `INSERT INTO fin_statement_templates
		(id, legal_entity_id, name, version, status, rows, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		t.ID, t.LegalEntityID, t.Name, t.Version, t.Status, t.Rows, t.CreatedBy)
	return err
}

func (r *FinModelRepository) GetStatementTemplate(ctx context.Context, id string) (*FinStatementTemplate, error) {
	return scanStatementTemplate(r.db.QueryRow(ctx, `SELECT `+finTemplateCols+` FROM fin_statement_templates WHERE id=$1`, id))
}

func (r *FinModelRepository) FindStatementTemplate(ctx context.Context, legalEntityID *string, name string, version int) (*FinStatementTemplate, error) {
	return scanStatementTemplate(r.db.QueryRow(ctx, `SELECT `+finTemplateCols+` FROM fin_statement_templates
		WHERE name=$1 AND version=$2 AND (legal_entity_id=$3 OR (legal_entity_id IS NULL AND $3::uuid IS NULL))
		ORDER BY created_at DESC LIMIT 1`, name, version, legalEntityID))
}

// Template governance transitions (S3-4): draft → review → approved, with
// the reviewer stamped on the middle hop. Each UPDATE is guarded by the
// current status so concurrent writers cannot skip a state; a zero-row
// result surfaces as the invalid-transition error the workflow mapping
// answers with 409. Retirement is deliberately absent — retiring a frozen
// template has no caller yet, and an unguarded path would be a hole.
func (r *FinModelRepository) ReviewStatementTemplate(ctx context.Context, id, reviewerID string, approved bool) error {
	stmt := `UPDATE fin_statement_templates SET status='review', reviewed_by=$2, reviewed_at=NOW()
		WHERE id=$1 AND status IN ('draft','review')`
	if !approved {
		stmt = `UPDATE fin_statement_templates SET status='draft', reviewed_by=$2, reviewed_at=NOW()
			WHERE id=$1 AND status='review'`
	}
	tag, err := r.db.Exec(ctx, stmt, id, reviewerID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrInvalidWorkflowTransition
	}
	return nil
}

func (r *FinModelRepository) ApproveStatementTemplate(ctx context.Context, id, approverID string) error {
	tag, err := r.db.Exec(ctx, `UPDATE fin_statement_templates
		SET status='approved', approved_by=$2, approved_at=NOW()
		WHERE id=$1 AND status='review'`, id, approverID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrInvalidWorkflowTransition
	}
	return nil
}

// ErrStatementTemplateInUse refuses deleting a template version that a
// model definition still binds to — deleting it would orphan live runs'
// structure and break their historical replay.
var ErrStatementTemplateInUse = errors.New("statement template is bound by a model definition and cannot be deleted")

// CopyStatementTemplate inserts a new draft version of the source's rows
// with the copy lineage set. Same-name copies continue the source lineage
// (version = sourceVersion+1); a new name starts a fresh lineage at
// version 1 — both are ordinary drafts and must pass review/approve like
// any creation.
func (r *FinModelRepository) CopyStatementTemplate(ctx context.Context, newID, name string, version int, sourceID, createdBy *string) error {
	_, err := r.db.Exec(ctx, `INSERT INTO fin_statement_templates
			(id, legal_entity_id, name, version, status, rows, created_by, copied_from)
		SELECT $1, legal_entity_id, $2, $3, 'draft', rows, $5, $4
		FROM fin_statement_templates WHERE id=$4`, newID, name, version, sourceID, createdBy)
	return err
}

// DeleteStatementTemplate removes a draft that was never used. The guards
// are the SQL WHERE: a non-draft status or any fin_model_definitions row
// bound to the template keeps it — history and replay stay intact.
func (r *FinModelRepository) DeleteStatementTemplate(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM fin_statement_templates
		WHERE id=$1 AND status='draft'
		  AND NOT EXISTS (SELECT 1 FROM fin_model_definitions WHERE template_id=$1)`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		// Distinguish the two refusal reasons for an honest 409.
		row, err := r.GetStatementTemplate(ctx, id)
		if err != nil || row == nil {
			return ErrInvalidWorkflowTransition
		}
		if row.Status != "draft" {
			return ErrInvalidWorkflowTransition
		}
		return ErrStatementTemplateInUse
	}
	return nil
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

const finRunCols = "id, legal_entity_id, model_definition_id, model_definition_version, data_version, assumption_version, exchange_rate_version, metric_definition_version, data_classification, status, tie_out_status, input_snapshot, idempotency_key, created_by, created_at, completed_at, failure_reason"

func scanModelRun(s rowScanner) (*FinModelRun, error) {
	var run FinModelRun
	err := s.Scan(&run.ID, &run.LegalEntityID, &run.ModelDefinitionID, &run.ModelDefinitionVersion,
		&run.DataVersion, &run.AssumptionVersion, &run.ExchangeRateVersion, &run.MetricDefinitionVersion,
		&run.DataClassification, &run.Status, &run.TieOutStatus, &run.InputSnapshot, &run.IdempotencyKey,
		&run.CreatedBy, &run.CreatedAt, &run.CompletedAt, &run.FailureReason)
	return &run, err
}

func (r *FinModelRepository) GetModelRun(ctx context.Context, id string) (*FinModelRun, error) {
	return scanModelRun(r.db.QueryRow(ctx, `SELECT `+finRunCols+` FROM fin_model_runs WHERE id=$1`, id))
}

// FindModelRunByIdempotency locates the run a replayed request already
// created — replays return the existing run's progress instead of a second
// record.
func (r *FinModelRepository) FindModelRunByIdempotency(ctx context.Context, definitionID, key string) (*FinModelRun, error) {
	run, err := scanModelRun(r.db.QueryRow(ctx, `SELECT id, legal_entity_id, model_definition_id, model_definition_version, data_version, assumption_version, exchange_rate_version, metric_definition_version, data_classification, status, tie_out_status, input_snapshot, idempotency_key, created_by, created_at, completed_at, failure_reason
		FROM fin_model_runs WHERE model_definition_id=$1 AND idempotency_key=$2
		ORDER BY created_at DESC LIMIT 1`, definitionID, key))
	if err != nil {
		return nil, err
	}
	return run, nil
}

func (r *FinModelRepository) UpdateModelRunStatus(ctx context.Context, id, status, tieOutStatus string, completedAt *time.Time) error {
	_, err := r.db.Exec(ctx, `UPDATE fin_model_runs SET status=$2, tie_out_status=$3, completed_at=$4 WHERE id=$1`,
		id, status, tieOutStatus, completedAt)
	return err
}

// CancelModelRun stops a queued/running run: the state guard is the WHERE
// so a completed run cannot be retro-cancelled and a cancelled run cannot
// be revived.
func (r *FinModelRepository) CancelModelRun(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `UPDATE fin_model_runs SET status='cancelled', completed_at=NOW()
		WHERE id=$1 AND status IN ('queued','running')`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrInvalidWorkflowTransition
	}
	return nil
}

// FailModelRun records an async failure with its reason.
func (r *FinModelRepository) FailModelRun(ctx context.Context, id, reason string) error {
	_, err := r.db.Exec(ctx, `UPDATE fin_model_runs SET status='failed', failure_reason=$2, completed_at=NOW()
		WHERE id=$1 AND status IN ('queued','running')`, id, reason)
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
