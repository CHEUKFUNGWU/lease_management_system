package aiintake

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func decodeJSON(reader io.Reader, destination interface{}) error {
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values are not allowed")
		}
		return fmt.Errorf("invalid trailing JSON: %w", err)
	}
	return nil
}

func validateRecordEvidence(evidence *Evidence, collection string, count int) error {
	for index := 0; index < count; index++ {
		recordField := collection
		if count > 1 || collection != "extracted_data" {
			recordField = fmt.Sprintf("%s[%d]", collection, index)
		}
		covered := false
		for _, locator := range evidence.Locators {
			if locator.Field == recordField || strings.HasPrefix(locator.Field, recordField+".") {
				covered = true
				break
			}
		}
		if !covered {
			return fmt.Errorf("complete AI intake evidence does not cover %s", recordField)
		}
	}
	return nil
}

func validateMetadata(metadata *IntakeMetadata, expectedDraftType string) error {
	if metadata.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported AI intake schema version %q", metadata.SchemaVersion)
	}
	if metadata.IntakeID == "" || metadata.TaskID == "" || metadata.FileID == "" {
		return fmt.Errorf("AI intake identity is incomplete")
	}
	if metadata.Mode != "assist" {
		return fmt.Errorf("AI intake mode %q is not allowed", metadata.Mode)
	}
	if metadata.DraftType != expectedDraftType {
		return fmt.Errorf("unexpected AI intake draft type %q", metadata.DraftType)
	}
	if metadata.Status != "draft_generated" {
		return fmt.Errorf("unexpected AI intake status %q", metadata.Status)
	}
	if metadata.Evidence.SourceFileID != metadata.FileID {
		return fmt.Errorf("AI intake evidence does not match source file")
	}
	if metadata.Evidence.ObjectName == "" || metadata.Evidence.ContentType == "" {
		return fmt.Errorf("AI intake source evidence is incomplete")
	}
	if metadata.Evidence.Complete && len(metadata.Evidence.Locators) == 0 {
		return fmt.Errorf("AI intake evidence is marked complete without locators")
	}
	if !metadata.Evidence.Complete && metadata.Evidence.MissingReason == "" {
		return fmt.Errorf("incomplete AI intake evidence requires a reason")
	}
	for index, locator := range metadata.Evidence.Locators {
		if locator.Field == "" || locator.Source == "" || locator.Quote == "" {
			return fmt.Errorf("AI intake evidence locator %d is incomplete", index)
		}
	}
	if !metadata.ReviewGate.Required {
		return fmt.Errorf("Assist Mode AI intake must require human review")
	}
	if !contains(metadata.ReviewGate.Reasons, "assist_mode") {
		return fmt.Errorf("Assist Mode AI intake review reasons must include assist_mode")
	}
	if metadata.ReviewGate.ConfidenceThreshold <= 0 || metadata.ReviewGate.ConfidenceThreshold > 1 {
		return fmt.Errorf("invalid AI intake confidence threshold %v", metadata.ReviewGate.ConfidenceThreshold)
	}
	if _, ok := metadata.ConfidenceScores["overall"]; !ok {
		return fmt.Errorf("AI intake confidence scores must include overall")
	}
	for name, confidence := range metadata.ConfidenceScores {
		if confidence < 0 || confidence > 1 {
			return fmt.Errorf("confidence score %q is outside [0,1]", name)
		}
	}
	return nil
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
