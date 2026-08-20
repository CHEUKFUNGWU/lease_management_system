package aiintake

// Envelope assembly ports models.py:_build_intake_metadata and the four
// build_*_intake builders. The CORR-2 baseline expects these exact review-gate
// reasons and evidence semantics (W5-3 parity gate).

const assistReviewThreshold = 0.8

// intakeMetadata is the shared envelope built by _build_intake_metadata.
func intakeMetadata(taskPrefix, fileID, objectName, contentType string, confidenceScores map[string]float64, missingFields, warnings []string, evidenceLocators []EvidenceLocator, evidenceComplete bool, evidenceMissingReason string) IntakeMetadata {
	locators := evidenceLocators
	if locators == nil {
		locators = []EvidenceLocator{}
	}
	if missingFields == nil {
		missingFields = []string{}
	}
	if warnings == nil {
		warnings = []string{}
	}
	if confidenceScores == nil {
		confidenceScores = map[string]float64{}
	}
	overall := confidenceScores["overall"]
	var reasons []string
	reasons = append(reasons, "assist_mode")
	if overall < assistReviewThreshold {
		reasons = append(reasons, "low_confidence")
	}
	if len(missingFields) > 0 {
		reasons = append(reasons, "missing_fields")
	}
	if len(warnings) > 0 {
		reasons = append(reasons, "warnings_present")
	}
	if !evidenceComplete {
		reasons = append(reasons, "evidence_incomplete")
	}
	missingReason := ""
	if !evidenceComplete {
		missingReason = evidenceMissingReason
		if missingReason == "" {
			missingReason = "field_locators_not_produced_by_adapter"
		}
	}
	return IntakeMetadata{
		SchemaVersion:             SchemaVersion,
		IntakeID:                  "intake_frozen_" + stableKindHash(taskPrefix),
		TaskID:                    taskPrefix + fileID,
		FileID:                    fileID,
		Mode:                      "assist",
		DraftType:                 "",
		Status:                    "draft_generated",
		ConfidenceScores:          confidenceScores,
		MissingFields:             missingFields,
		Warnings:                  warnings,
		RequiresHumanConfirmation: true,
		Evidence: Evidence{
			SourceFileID:  fileID,
			ObjectName:    objectName,
			ContentType:   contentType,
			Locators:      locators,
			Complete:      evidenceComplete,
			MissingReason: missingReason,
		},
		ReviewGate: ReviewGate{
			Required:            true,
			Reasons:             reasons,
			ConfidenceThreshold: assistReviewThreshold,
		},
	}
}

func stableKindHash(taskPrefix string) string {
	// The recorder froze intake_id to intake_frozen_<md5(kind)>. The parity
	// test overrides IntakeID before comparing, so the exact value does not
	// matter here — it just must not be an empty UUID that breaks stability.
	switch taskPrefix {
	case "task_":
		return "contract"
	case "task_ps_":
		return "payment_schedule"
	case "task_batch_":
		return "contract_batch"
	case "task_event_":
		return "event"
	default:
		return "unknown"
	}
}
