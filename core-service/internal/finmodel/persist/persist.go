// Package persist is SM2's single persistence entry point (D-S2): the pure
// engine lives in the parent package and never touches the repository; this
// package exists so the import guard over the engine stays meaningful and
// exactly one write path exists for fin_model_runs.
package persist

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lease-management-system/core-service/internal/finmodel"
	"github.com/lease-management-system/core-service/internal/repository"
)

// ErrTieOutFailed is the publish gate (fail-closed #1): a run whose tie-outs
// fail must never persist as a publishable result (S2-6 / D-S5).
var ErrTieOutFailed = errors.New("finmodel: run refused — tie-outs failed and must not be published")

// RunWriter is the single write entry of the model.
type RunWriter struct {
	repo *repository.FinModelRepository
	now  func() time.Time
}

// NewRunWriter builds the writer over the repository.
func NewRunWriter(repo *repository.FinModelRepository) *RunWriter {
	return &RunWriter{repo: repo, now: time.Now}
}

// Persist stores the completed run. A failed tie-out status is an error —
// never a persisted "working" result. Replays hit the repository's unique
// (model_definition_id, idempotency_key) guard.
func (w *RunWriter) Persist(ctx context.Context, def finmodel.ModelDef, result *finmodel.RunResult, modelDefinitionID string, idempotencyKey string, createdBy *string) error {
	if result.TieOutStatus == "failed" {
		return ErrTieOutFailed
	}
	if w.repo == nil {
		return errors.New("finmodel persist: repository unavailable")
	}
	rawSnapshot, err := Snapshot(def)
	if err != nil {
		return err
	}
	runID := uuid.NewString()
	now := w.now()
	run := &repository.FinModelRun{
		ID: runID, LegalEntityID: def.LegalEntityID, ModelDefinitionID: modelDefinitionID,
		ModelDefinitionVersion:  1,
		DataVersion:             strPtr(result.Versions.Data),
		AssumptionVersion:       strPtr(result.Versions.Assumption),
		ExchangeRateVersion:     strPtr(result.Versions.ExchangeRate),
		MetricDefinitionVersion: strPtr(result.Versions.MetricDefinition),
		DataClassification:      result.Versions.Data,
		Status:                  "completed",
		TieOutStatus:            result.TieOutStatus,
		InputSnapshot:           rawSnapshot,
		IdempotencyKey:          idempotencyKey,
		CreatedBy:               createdBy,
		CreatedAt:               now,
		CompletedAt:             &now,
	}
	if err := w.repo.CreateModelRun(ctx, run); err != nil {
		return fmt.Errorf("persist finmodel run: %w", err)
	}
	return w.persistResult(ctx, runID, result, now)
}

// Snapshot renders the input snapshot a run row stores — the same shape the
// sync and async paths both persist, so replays compare like for like.
func Snapshot(def finmodel.ModelDef) (json.RawMessage, error) {
	snapshot := map[string]any{
		"model_name": def.Name, "currency": def.Currency, "period_start": def.PeriodStart,
		"historical_months": def.HistoricalMonths, "forecast_months": def.ForecastMonths,
		"actual_cutoff_period": def.ActualCutoffPeriod, "policy": def.Policy,
	}
	return json.Marshal(snapshot)
}

// PersistInto is the async sibling (S2-5): the run row already exists in
// queued state, so only the result lines, tie-outs and the completion
// status are written. The same tie-out gate applies — an async run whose
// tie-outs fail completes as failed, never as a publishable result.
func (w *RunWriter) PersistInto(ctx context.Context, runID string, result *finmodel.RunResult) error {
	if result.TieOutStatus == "failed" {
		return ErrTieOutFailed
	}
	if w.repo == nil {
		return errors.New("finmodel persist: repository unavailable")
	}
	now := w.now()
	if err := w.persistResult(ctx, runID, result, now); err != nil {
		return err
	}
	return w.repo.UpdateModelRunStatus(ctx, runID, "completed", result.TieOutStatus, &now)
}

func (w *RunWriter) persistResult(ctx context.Context, runID string, result *finmodel.RunResult, now time.Time) error {
	lines := make([]repository.FinModelRunLine, 0, len(result.Lines))
	for _, line := range result.Lines {
		prov, err := json.Marshal(line.Provenance)
		if err != nil {
			return err
		}
		lines = append(lines, repository.FinModelRunLine{
			RunID: runID, RowKey: line.RowKey, Period: line.Period,
			Value: line.Value, Provenance: prov,
		})
	}
	if err := w.repo.InsertRunLines(ctx, lines); err != nil {
		return err
	}
	outs := make([]repository.FinModelTieOut, 0, len(result.TieOuts))
	for _, out := range result.TieOuts {
		outs = append(outs, repository.FinModelTieOut{
			RunID: runID, CheckCode: out.CheckCode, Period: out.Period,
			Expected: out.Expected, Actual: out.Actual, Diff: out.Diff, Status: out.Status,
		})
	}
	return w.repo.InsertTieOuts(ctx, outs)
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
