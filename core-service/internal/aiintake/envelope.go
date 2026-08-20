package aiintake

// Envelope builders turn a draft's normalized data into the exact ai-intake.v1
// wire map (contract / payment_schedule / contract_batch / event), mirroring
// models.py builders so the CORR-2 parity comparison is structural.

func intakeEnvelopeMap(taskPrefix, fileID, objectName, contentType, draftType string, confidence map[string]float64, missing, warnings []string, locators []EvidenceLocator, complete bool, missingReason string, payload map[string]any) map[string]any {
	meta := intakeMetadata(taskPrefix, fileID, objectName, contentType, confidence, missing, warnings, locators, complete, missingReason)
	meta.DraftType = draftType
	evidence := map[string]any{
		"source_file_id": meta.Evidence.SourceFileID,
		"object_name":    meta.Evidence.ObjectName,
		"content_type":   meta.Evidence.ContentType,
		"locators":       meta.Evidence.Locators,
		"complete":       meta.Evidence.Complete,
		"missing_reason": nilIf(meta.Evidence.MissingReason),
	}
	env := map[string]any{
		"schema_version":              meta.SchemaVersion,
		"intake_id":                   meta.IntakeID,
		"task_id":                     meta.TaskID,
		"file_id":                     meta.FileID,
		"mode":                        "assist",
		"status":                      "draft_generated",
		"confidence_scores":           meta.ConfidenceScores,
		"missing_fields":              meta.MissingFields,
		"warnings":                    meta.Warnings,
		"requires_human_confirmation": true,
		"evidence":                    evidence,
		"review_gate": map[string]any{
			"required":             true,
			"reasons":              meta.ReviewGate.Reasons,
			"confidence_threshold": meta.ReviewGate.ConfidenceThreshold,
		},
		"draft_type": draftType,
	}
	for k, v := range payload {
		env[k] = v
	}
	return env
}

func nilIf(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func contractEnvelope(cmd IntakeCommandLike, normalized map[string]any, confidence map[string]float64, missing, warnings []string, verified, materialLocators []EvidenceLocator, complete bool, missingReason string) map[string]any {
	locators := verified
	if len(locators) == 0 {
		locators = materialLocators
	}
	return intakeEnvelopeMap("task_", cmd.FileID(), cmd.ObjectName(), cmd.ContentType(), "contract_draft",
		confidence, missing, warnings, locators, complete, missingReason,
		map[string]any{"extracted_data": normalized})
}

func paymentEnvelope(cmd IntakeCommandLike, schedules []PaymentSchedule, confidence map[string]float64, missing, warnings []string, verified, materialLocators []EvidenceLocator, complete bool, missingReason string) map[string]any {
	locators := verified
	if len(locators) == 0 {
		locators = materialLocators
	}
	if schedules == nil {
		schedules = []PaymentSchedule{}
	}
	return intakeEnvelopeMap("task_ps_", cmd.FileID(), cmd.ObjectName(), cmd.ContentType(), "payment_schedule_draft",
		confidence, missing, warnings, locators, complete, missingReason,
		map[string]any{"schedules": schedules})
}

func eventEnvelope(cmd IntakeCommandLike, event EventDraftData, confidence map[string]float64, missing, warnings []string, verified, materialLocators []EvidenceLocator, complete bool, missingReason string) map[string]any {
	locators := verified
	if len(locators) == 0 {
		locators = materialLocators
	}
	// The old path emits null (not absent) for absent original/new values.
	eventMap := map[string]any{
		"contract_id":         event.ContractID,
		"contract_number":     event.ContractNumber,
		"event_type":          event.EventType,
		"effective_date":      event.EffectiveDate,
		"original_value":      anyStringOrNil(event.OriginalValue),
		"new_value":           anyStringOrNil(event.NewValue),
		"change_reason":       event.ChangeReason,
		"judgment_basis":      event.JudgmentBasis,
		"revision_parameters": event.RevisionParameters,
		"field_confidence":    event.FieldConfidence,
	}
	return intakeEnvelopeMap("task_event_", cmd.FileID(), cmd.ObjectName(), cmd.ContentType(), "event_draft",
		confidence, missing, warnings, locators, complete, missingReason,
		map[string]any{"event": eventMap})
}

func anyStringOrNil(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

func batchEnvelope(cmd IntakeCommandLike, contracts []map[string]any, confidence map[string]float64, missing, warnings []string, locators []EvidenceLocator, complete bool, missingReason string) map[string]any {
	if contracts == nil {
		contracts = []map[string]any{}
	}
	return intakeEnvelopeMap("task_batch_", cmd.FileID(), cmd.ObjectName(), cmd.ContentType(), "contract_batch_draft",
		confidence, missing, warnings, locators, complete, missingReason,
		map[string]any{"contracts": contracts, "total_count": len(contracts)})
}
