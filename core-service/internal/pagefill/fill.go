// Package pagefill is the page-fill seam (Agent Core design appendix A):
// the protocol an Agent uses to prefill a functional page, and the one asset
// every commit path must respect — prefill, never commit; commit is always
// human-driven. The core invariant: values carrying an Exploratory basis may
// live only in Suggestions; Payload is constructor-guarded so an Exploratory
// value structurally cannot reach a commit (invariant I5 / ACORE-12).
package pagefill

import (
	"errors"
	"fmt"
	"strings"

	"github.com/lease-management-system/core-service/internal/workingpaper"
)

// SchemaVersion is the page-fill protocol version.
const SchemaVersion = "page-fill.v1"

// FillValue is one prefill field with the same provenance structure as
// working-paper cells (appendix A.3: one trace chain, two paths).
type FillValue struct {
	Value      any                     `json:"value"`
	Provenance workingpaper.Provenance `json:"provenance"`
}

// Fill is one page prefill: confirmed/system values in Payload, unconfirmed
// suggestions (mostly Exploratory) in Suggestions.
type Fill struct {
	SchemaVersion  string               `json:"schema_version"`
	TargetPage     string               `json:"target_page"`
	TargetAPI      string               `json:"target_api"`
	DeepLink       string               `json:"deep_link"`
	Payload        map[string]FillValue `json:"payload"`
	Suggestions    map[string]FillValue `json:"suggestions,omitempty"`
	Confidence     float64              `json:"confidence"`
	ReviewRequired bool                 `json:"review_required"`
	ReviewReasons  []string             `json:"review_reasons,omitempty"`
}

// New builds an empty fill for a target page.
func New(targetPage, targetAPI, deepLink string) *Fill {
	return &Fill{
		SchemaVersion:  SchemaVersion,
		TargetPage:     targetPage,
		TargetAPI:      targetAPI,
		DeepLink:       deepLink,
		Payload:        map[string]FillValue{},
		Suggestions:    map[string]FillValue{},
		ReviewRequired: true,
	}
}

// PutPayload stores a value that may never be Exploratory. This is the only
// unmediated write into Payload besides Confirm — and it refuses Exploratory
// at the gate.
func (f *Fill) PutPayload(field string, value any, p workingpaper.Provenance) error {
	if p.Basis == workingpaper.BasisExploratory {
		return fmt.Errorf("pagefill: field %q has Exploratory basis and cannot enter the payload (I5)", field)
	}
	f.Payload[field] = FillValue{Value: value, Provenance: p}
	return nil
}

// Suggest records an unconfirmed suggestion. Exploratory lives here.
func (f *Fill) Suggest(field string, value any, p workingpaper.Provenance) {
	f.Suggestions[field] = FillValue{Value: value, Provenance: p}
}

// Confirm promotes a suggestion into the payload as a human-confirmed value.
// It is the confirm seam: the confirmed basis flips to HumanInput with the
// confirmer's identity, so every confirmed field answers "who confirmed it".
func (f *Fill) Confirm(field string, value any, confirmedBy, confirmedAt string) error {
	if strings.TrimSpace(confirmedBy) == "" {
		return errors.New("pagefill: confirmed_by is required")
	}
	if _, exists := f.Suggestions[field]; !exists {
		return fmt.Errorf("pagefill: field %q has no pending suggestion to confirm", field)
	}
	f.Payload[field] = FillValue{Value: value, Provenance: workingpaper.Provenance{
		Basis:       workingpaper.BasisHumanInput,
		ConfirmedBy: confirmedBy,
		ConfirmedAt: confirmedAt,
	}}
	delete(f.Suggestions, field)
	return nil
}

// ExploratoryRefs lists the fields whose provenance is Exploratory — the
// source for the I5 write-path assertion.
func (f *Fill) ExploratoryRefs() []string {
	var out []string
	for field, v := range f.Suggestions {
		if v.Provenance.Basis == workingpaper.BasisExploratory {
			out = append(out, field)
		}
	}
	for field, v := range f.Payload {
		if v.Provenance.Basis == workingpaper.BasisExploratory {
			out = append(out, field)
		}
	}
	return out
}

// AssertNoExploratoryInPayload enforces ACORE-12 at the commit seam: an
// Exploratory value inside the payload is an immediate refusal.
func (f *Fill) AssertNoExploratoryInPayload() error {
	for field, v := range f.Payload {
		if v.Provenance.Basis == workingpaper.BasisExploratory {
			return fmt.Errorf("pagefill: field %q in payload carries an Exploratory basis — commit refused (I5)", field)
		}
	}
	return nil
}

// Validate checks the envelope: target page and API present, payload values
// carry a valid provenance basis.
func (f Fill) Validate() error {
	if strings.TrimSpace(f.TargetPage) == "" || strings.TrimSpace(f.TargetAPI) == "" {
		return errors.New("pagefill: target_page and target_api are required")
	}
	if f.SchemaVersion != SchemaVersion {
		return fmt.Errorf("pagefill: schema_version must be %s", SchemaVersion)
	}
	for field, v := range f.Payload {
		if !v.Provenance.Basis.Valid() {
			return fmt.Errorf("pagefill: field %q has invalid provenance basis %q", field, v.Provenance.Basis)
		}
	}
	return nil
}

// FillPayload is the wire projection for the page-fill artifact data.
func (f *Fill) FillPayload() map[string]any {
	payload := map[string]any{}
	for field, v := range f.Payload {
		payload[field] = v.Value
	}
	return payload
}
