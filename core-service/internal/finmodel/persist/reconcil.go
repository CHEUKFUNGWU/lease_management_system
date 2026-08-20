package persist

// P1-1: the reconciliation ledger. A run whose Actual window disagrees with
// the fact layer (T13) or whose opening gate③ lease balances differ must
// neither publish nor vanish — the mismatch goes to fpna_data_quality_items
// (category=reconciliation) with its source table / record / version, so the
// queue is the visible place an un-reconcilable run lives until a human acts.

import (
	"context"
	"fmt"

	"github.com/lease-management-system/core-service/internal/finmodel"
	"github.com/lease-management-system/core-service/internal/finmodel/opening"
	"github.com/lease-management-system/core-service/internal/repository"
)

// RecordReconciliationIssues writes one data-quality row per failed T13
// check. The failing run never publishes (S2-6), but the mismatch must stay
// visible. sourceRecordID traces to the run submission; dataVersion rides the
// run's own version line (PRD T13: 不符进 data_quality_items，不调平).
func RecordReconciliationIssues(ctx context.Context, repo *repository.FinModelRepository, legalEntityID, sourceRecordID, dataVersion string, tieOuts []finmodel.TieOutResult) error {
	if repo == nil {
		return nil
	}
	written := 0
	for _, out := range tieOuts {
		if out.CheckCode != "T13" || out.Status != "failed" {
			continue
		}
		if err := repo.InsertReconciliationIssue(ctx, legalEntityID, "fin_model_tie_outs", sourceRecordID, out.Period, dataVersion,
			fmt.Sprintf("T13 @ %s Actual 与事实层不一致（期望 %v，实际 %v）", out.Period, fmtFloat(out.Expected), fmtFloat(out.Actual))); err != nil {
			return err
		}
		written++
	}
	_ = written
	return nil
}

// RecordOpeningGateIssues queues the three-gate failures that rejected the
// run — gate③ (lease reconciliation against the engine) is the PRD-named
// one, but the queue records every gate failure so the import problem is
// addressed on the queue, not swallowed by the refusal.
func RecordOpeningGateIssues(ctx context.Context, repo *repository.FinModelRepository, legalEntityID, sourceRecordID string, failures []opening.GateFailure) error {
	if repo == nil {
		return nil
	}
	for _, f := range failures {
		if err := repo.InsertReconciliationIssue(ctx, legalEntityID, "fin_model_opening", sourceRecordID, f.Period, "",
			fmt.Sprintf("opening gate %s: %s", f.Gate, f.Detail)); err != nil {
			return err
		}
	}
	return nil
}

func fmtFloat(v *float64) string {
	if v == nil {
		return "nil"
	}
	return fmt.Sprintf("%v", *v)
}
