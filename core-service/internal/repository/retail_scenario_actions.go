package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrRetailScenarioActionScopeConflict indicates an existing legacy/action row
// occupies the same scope without the idempotency key being available. The
// scenario handler maps this to a stable 409 rather than returning a caller
// generated UUID that was never persisted.
var ErrRetailScenarioActionScopeConflict = errors.New("retail scenario action scope conflict")

// CreateScenarioAction is additive to CreateAction. Scenario drafts use a
// fingerprint-derived rule_code and a single-statement DO NOTHING insert so
// concurrent requests with the same idempotency key converge on the row that
// actually won the database unique index. Existing action APIs retain their
// historical scope-update semantics.
func (r *OperatingFactsRepository) CreateScenarioAction(ctx context.Context, item *FPnAActionItem) (*FPnAActionItem, bool, error) {
	if item.ID == "" {
		item.ID = uuid.NewString()
	}
	if item.Evidence == nil {
		item.Evidence = json.RawMessage(`{}`)
	}
	if item.Status == "" {
		item.Status = "open"
	}
	if item.VerificationStatus == "" {
		item.VerificationStatus = "not_due"
	}
	var createdAt, updatedAt time.Time
	err := r.db.QueryRow(ctx, `
		INSERT INTO fpna_action_items
		(id,legal_entity_id,period,category,severity,status,title,description,rule_code,source_table,source_record_id,data_version,idempotency_key,impact_amount,currency,owner_id,owner_name,due_date,baseline_amount,target_amount,expected_benefit,verification_period,verified_amount,verification_status,human_root_cause,planned_action,ai_suggestion,evidence,created_by,updated_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NULLIF($13,''),$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30)
		ON CONFLICT DO NOTHING
		RETURNING created_at,updated_at`, item.ID, item.LegalEntityID, item.Period, item.Category, item.Severity, item.Status, item.Title, item.Description, item.RuleCode, item.SourceTable, item.SourceRecordID, item.DataVersion, item.IdempotencyKey, item.ImpactAmount, item.Currency, item.OwnerID, item.OwnerName, item.DueDate, item.BaselineAmount, item.TargetAmount, item.ExpectedBenefit, item.VerificationPeriod, item.VerifiedAmount, item.VerificationStatus, item.HumanRootCause, item.PlannedAction, item.AISuggestion, item.Evidence, item.CreatedBy, item.UpdatedBy).Scan(&createdAt, &updatedAt)
	if err == nil {
		item.CreatedAt = createdAt
		item.UpdatedAt = updatedAt
		return item, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, err
	}
	if item.IdempotencyKey == "" {
		return nil, false, ErrRetailScenarioActionScopeConflict
	}
	filter, filterErr := entityFilterForOptional(item.LegalEntityID)
	if filterErr != nil {
		return nil, false, filterErr
	}
	existing, lookupErr := r.GetActionByIdempotency(ctx, filter, item.IdempotencyKey)
	if lookupErr != nil {
		return nil, false, lookupErr
	}
	if existing == nil {
		return nil, false, ErrRetailScenarioActionScopeConflict
	}
	return existing, true, nil
}
