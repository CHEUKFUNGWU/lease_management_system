package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/services/closecontrol"
	"github.com/lease-management-system/core-service/internal/services/controlrules"
)

type CloseControlRepository struct {
	db DBTX
}

func NewCloseControlRepository(db DBTX) *CloseControlRepository {
	return &CloseControlRepository{db: db}
}

func (r *CloseControlRepository) GetActiveRule(ctx context.Context, code string, asOf time.Time) (controlrules.Definition, bool, error) {
	var rule controlrules.Definition
	err := r.db.QueryRow(ctx, `
		SELECT rule_code, rule_version, name, severity, gate_effect, title,
			reason_template, remediation
		FROM close_control_rules
		WHERE rule_code = $1
		  AND enabled = true
		  AND effective_from <= $2::date
		  AND (effective_to IS NULL OR effective_to >= $2::date)
		ORDER BY effective_from DESC, created_at DESC
		LIMIT 1
	`, code, asOf).Scan(
		&rule.Code, &rule.Version, &rule.Name, &rule.Severity, &rule.GateEffect,
		&rule.Title, &rule.ReasonTemplate, &rule.Remediation,
	)
	if err == pgx.ErrNoRows {
		return controlrules.Definition{}, false, nil
	}
	if err != nil {
		return controlrules.Definition{}, false, fmt.Errorf("load active close control rule: %w", err)
	}
	return rule, true, nil
}

func (r *CloseControlRepository) PersistDetections(ctx context.Context, detections []closecontrol.Detection) ([]closecontrol.Exception, error) {
	result := make([]closecontrol.Exception, 0, len(detections))
	for _, detection := range detections {
		evidence, err := json.Marshal(detection.Evidence)
		if err != nil {
			return nil, fmt.Errorf("marshal close detection evidence: %w", err)
		}
		var detectionID string
		var legalEntity interface{}
		if detection.LegalEntityID != "" {
			legalEntity = detection.LegalEntityID
		}
		var subjectContract interface{}
		if detection.SubjectContractID != "" {
			subjectContract = detection.SubjectContractID
		}
		err = r.db.QueryRow(ctx, `
			INSERT INTO close_detection_events (
				control_rule_id, rule_code, rule_version, legal_entity_id, accounting_period,
				projection_version, subject_type, subject_id, subject_contract_id,
				fingerprint, evidence, detected_at
			)
			SELECT id, $1::varchar, $2::varchar, $3::uuid, $4, $5, $6, $7::uuid, $8::uuid, $9, $10::jsonb, $11
			FROM close_control_rules
			WHERE rule_code = $1::varchar AND rule_version = $2::varchar AND enabled = true
			RETURNING id::text
		`, detection.RuleCode, detection.RuleVersion, legalEntity, detection.AccountingPeriod,
			detection.ProjectionVersion, detection.SubjectType, detection.SubjectID, subjectContract,
			detection.Fingerprint, evidence, detection.DetectedAt).Scan(&detectionID)
		if err != nil {
			return nil, fmt.Errorf("persist close detection event: %w", err)
		}

		var exceptionID string
		err = r.db.QueryRow(ctx, `
			INSERT INTO close_exceptions (
				detection_event_id, fingerprint, rule_code, rule_version, severity, gate_effect,
				legal_entity_id, accounting_period, subject_type, subject_id, subject_contract_id,
				projection_version, exception_state, closing_disposition, opened_at,
				last_detected_at, updated_at
			) VALUES ($1::uuid, $2, $3, $4, $5, $6, $7::uuid, $8, $9, $10::uuid, $11::uuid,
				$12, 'open', 'unresolved', $13, $13, $13)
			ON CONFLICT (fingerprint) DO UPDATE SET
				detection_event_id = EXCLUDED.detection_event_id,
				last_detected_at = EXCLUDED.last_detected_at,
				projection_version = EXCLUDED.projection_version,
				updated_at = EXCLUDED.updated_at
			RETURNING id::text
		`, detectionID, detection.Fingerprint, detection.RuleCode, detection.RuleVersion,
			detection.Severity, detection.GateEffect, legalEntity, detection.AccountingPeriod,
			detection.SubjectType, detection.SubjectID, subjectContract, detection.ProjectionVersion,
			detection.DetectedAt).Scan(&exceptionID)
		if err != nil {
			return nil, fmt.Errorf("persist close exception: %w", err)
		}
		exception, err := r.GetException(ctx, exceptionID)
		if err != nil {
			return nil, err
		}
		if exception != nil {
			result = append(result, *exception)
		}
	}
	return result, nil
}

func (r *CloseControlRepository) ListExceptions(ctx context.Context, period, legalEntityID string) ([]closecontrol.Exception, error) {
	query, args := r.exceptionQuery(`
		SELECT ce.id::text, ce.detection_event_id::text, ce.rule_code, ce.rule_version,
			ce.severity, ce.gate_effect, ce.accounting_period, COALESCE(ce.legal_entity_id::text, ''),
			ce.subject_type, ce.subject_id::text, ce.subject_contract_id::text,
			COALESCE(c.contract_number, ''), COALESCE(c.contract_name, ''), COALESCE(b.batch_number, ''),
			ce.fingerprint, ce.projection_version, de.evidence,
			ce.exception_state, ce.closing_disposition, ce.owner_id::text, ce.reviewer_id::text,
			ce.approver_id::text, ce.opened_at, ce.last_detected_at, ce.investigating_at,
			ce.resolved_at, ce.waived_at, ce.closed_at, ce.resolution_note, ce.updated_at
		FROM close_exceptions ce
		JOIN close_detection_events de ON de.id = ce.detection_event_id
		LEFT JOIN lease_contracts c ON c.id = ce.subject_contract_id
		LEFT JOIN monthly_closing_batches b ON b.id = ce.subject_id
		WHERE ce.accounting_period = $1`, []interface{}{period})
	if legalEntityID != "" {
		query += fmt.Sprintf(" AND ce.legal_entity_id = $%d", len(args)+1)
		args = append(args, legalEntityID)
	}
	query, args = appendExceptionScope(ctx, query, args)
	query += " ORDER BY CASE WHEN ce.exception_state = 'closed' THEN 1 ELSE 0 END, ce.severity DESC, ce.opened_at DESC, ce.id"
	return r.queryExceptions(ctx, query, args...)
}

func (r *CloseControlRepository) GetException(ctx context.Context, id string) (*closecontrol.Exception, error) {
	query, args := r.exceptionQuery(`
		SELECT ce.id::text, ce.detection_event_id::text, ce.rule_code, ce.rule_version,
			ce.severity, ce.gate_effect, ce.accounting_period, COALESCE(ce.legal_entity_id::text, ''),
			ce.subject_type, ce.subject_id::text, ce.subject_contract_id::text,
			COALESCE(c.contract_number, ''), COALESCE(c.contract_name, ''), COALESCE(b.batch_number, ''),
			ce.fingerprint, ce.projection_version, de.evidence,
			ce.exception_state, ce.closing_disposition, ce.owner_id::text, ce.reviewer_id::text,
			ce.approver_id::text, ce.opened_at, ce.last_detected_at, ce.investigating_at,
			ce.resolved_at, ce.waived_at, ce.closed_at, ce.resolution_note, ce.updated_at
		FROM close_exceptions ce
		JOIN close_detection_events de ON de.id = ce.detection_event_id
		LEFT JOIN lease_contracts c ON c.id = ce.subject_contract_id
		LEFT JOIN monthly_closing_batches b ON b.id = ce.subject_id
		WHERE ce.id = $1`, []interface{}{id})
	query, args = appendExceptionScope(ctx, query, args)
	var exceptions []closecontrol.Exception
	var err error
	exceptions, err = r.queryExceptions(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	if len(exceptions) == 0 {
		return nil, closecontrol.ErrNotFound
	}
	return &exceptions[0], nil
}

func (r *CloseControlRepository) UpdateException(ctx context.Context, id string, update closecontrol.ExceptionUpdate) (*closecontrol.Exception, error) {
	var ownerID, reviewerID, approverID interface{}
	if update.OwnerID != nil && *update.OwnerID != "" {
		ownerID = *update.OwnerID
	}
	if update.ReviewerID != nil && *update.ReviewerID != "" {
		reviewerID = *update.ReviewerID
	}
	if update.ApproverID != nil && *update.ApproverID != "" {
		approverID = *update.ApproverID
	}
	_, err := r.db.Exec(ctx, `
		UPDATE close_exceptions SET
			exception_state = $1, closing_disposition = $2,
			owner_id = $3::uuid, reviewer_id = $4::uuid, approver_id = $5::uuid,
			resolution_note = $6, investigating_at = $7, resolved_at = $8,
			waived_at = $9, closed_at = $10, updated_at = NOW()
		WHERE id = $11::uuid
	`, update.ExceptionState, update.ClosingDisposition, ownerID, reviewerID, approverID,
		update.ResolutionNote, update.InvestigatingAt, update.ResolvedAt, update.WaivedAt, update.ClosedAt, id)
	if err != nil {
		return nil, fmt.Errorf("update close exception: %w", err)
	}
	return r.GetException(ctx, id)
}

func (r *CloseControlRepository) HasUnresolvedBlocking(ctx context.Context, period, legalEntityID string) (bool, error) {
	var legalEntity interface{}
	condition := "TRUE"
	if legalEntityID != "" {
		legalEntity = legalEntityID
		condition = "legal_entity_id = $2::uuid"
	}
	var found bool
	err := r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT EXISTS (
			SELECT 1 FROM close_exceptions
			WHERE accounting_period = $1
			  AND %s
			  AND severity = 'blocking'
			  AND NOT (
					exception_state = 'closed'
					AND closing_disposition IN ('verified_resolution', 'accounting_conclusion', 'period_waiver', 'standing_waiver')
				)
		)`, condition), period, legalEntity).Scan(&found)
	if err != nil {
		return false, fmt.Errorf("check unresolved close exceptions: %w", err)
	}
	return found, nil
}

func (r *CloseControlRepository) exceptionQuery(selectSQL string, args []interface{}) (string, []interface{}) {
	return selectSQL, args
}

func appendExceptionScope(ctx context.Context, query string, args []interface{}) (string, []interface{}) {
	scope, scoped := access.ScopeFromContext(ctx)
	if !scoped || scope.Global || (len(scope.StoreIDs) == 0 && len(scope.Regions) == 0 && len(scope.Brands) == 0) {
		return query, args
	}
	conditions := []string{"c.id IS NOT NULL"}
	if scope.LegalEntityID != "" {
		conditions = append(conditions, fmt.Sprintf("c.legal_entity_id::text = $%d", len(args)+1))
		args = append(args, scope.LegalEntityID)
	}
	if len(scope.StoreIDs) > 0 {
		conditions = append(conditions, fmt.Sprintf("c.store_id::text = ANY($%d)", len(args)+1))
		args = append(args, scope.StoreIDs)
	}
	if len(scope.Regions) > 0 {
		conditions = append(conditions, fmt.Sprintf("c.store_id IN (SELECT id FROM stores WHERE region = ANY($%d))", len(args)+1))
		args = append(args, scope.Regions)
	}
	if len(scope.Brands) > 0 {
		conditions = append(conditions, fmt.Sprintf("c.store_id IN (SELECT id FROM stores WHERE brand = ANY($%d))", len(args)+1))
		args = append(args, scope.Brands)
	}
	return query + " AND (" + strings.Join(conditions, " AND ") + ")", args
}

func (r *CloseControlRepository) queryExceptions(ctx context.Context, query string, args ...interface{}) ([]closecontrol.Exception, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query close exceptions: %w", err)
	}
	defer rows.Close()
	result := make([]closecontrol.Exception, 0)
	for rows.Next() {
		var exception closecontrol.Exception
		var evidence []byte
		var subjectContractID, ownerID, reviewerID, approverID, resolutionNote *string
		if err := rows.Scan(
			&exception.ID, &exception.DetectionEventID, &exception.RuleCode, &exception.RuleVersion,
			&exception.Severity, &exception.GateEffect, &exception.AccountingPeriod, &exception.LegalEntityID,
			&exception.SubjectType, &exception.SubjectID, &subjectContractID, &exception.ContractNumber,
			&exception.ContractName, &exception.BatchNumber, &exception.Fingerprint, &exception.ProjectionVersion,
			&evidence, &exception.ExceptionState, &exception.ClosingDisposition, &ownerID, &reviewerID,
			&approverID, &exception.OpenedAt, &exception.LastDetectedAt, &exception.InvestigatingAt,
			&exception.ResolvedAt, &exception.WaivedAt, &exception.ClosedAt, &resolutionNote, &exception.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan close exception: %w", err)
		}
		exception.SubjectContractID = subjectContractID
		exception.OwnerID, exception.ReviewerID, exception.ApproverID = ownerID, reviewerID, approverID
		exception.ResolutionNote = resolutionNote
		if len(evidence) > 0 {
			var decoded any
			if err := json.Unmarshal(evidence, &decoded); err == nil {
				exception.Evidence = decoded
			}
		}
		result = append(result, exception)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate close exceptions: %w", err)
	}
	return result, nil
}
