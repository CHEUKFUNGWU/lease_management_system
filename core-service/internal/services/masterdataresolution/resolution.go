package masterdataresolution

import (
	"context"
	"strings"
)

type EntityKind string

const (
	KindStore    EntityKind = "store"
	KindSKU      EntityKind = "sku"
	KindCategory EntityKind = "category"
)

type Candidate struct {
	RawIdentifier string  `json:"raw_identifier"`
	CanonicalID   string  `json:"canonical_id"`
	CanonicalName string  `json:"canonical_name"`
	Confidence    float64 `json:"confidence"`
	Source        string  `json:"source"` // "cached", "rule", "ai"
}

type Resolution struct {
	Resolved  map[string]Candidate `json:"resolved"`  // raw -> candidate
	Unknown   []string             `json:"unknown"`   // raw identifiers not resolved
	Ambiguous []Candidate          `json:"ambiguous"` // candidates with confidence < threshold
}

type PersistentMappingReader interface {
	GetConfirmedMappings(ctx context.Context, kind EntityKind, raws []string) (map[string]Candidate, error)
}

type SuggestionSource interface {
	Suggest(ctx context.Context, kind EntityKind, unknownRaws []string) ([]Candidate, error)
}

// RuleSuggestionSource provides deterministic exact & trimmed prefix matching.
type RuleSuggestionSource struct {
	Dictionary map[string]string // raw/alias -> canonical_id
}

func (s *RuleSuggestionSource) Suggest(ctx context.Context, kind EntityKind, unknownRaws []string) ([]Candidate, error) {
	var candidates []Candidate
	for _, raw := range unknownRaws {
		trimmed := strings.TrimSpace(raw)
		if canon, ok := s.Dictionary[trimmed]; ok {
			candidates = append(candidates, Candidate{
				RawIdentifier: raw,
				CanonicalID:   canon,
				CanonicalName: canon,
				Confidence:    1.0,
				Source:        "rule",
			})
		}
	}
	return candidates, nil
}

// Resolve executes two-phase resolution:
// Phase 1: Check persistent confirmed cache.
// Phase 2: Call SuggestionSource ONLY on unknown delta (AI cost convergence).
func Resolve(
	ctx context.Context,
	kind EntityKind,
	raws []string,
	cache PersistentMappingReader,
	source SuggestionSource,
	confidenceThreshold float64,
) (Resolution, error) {
	if confidenceThreshold <= 0 {
		confidenceThreshold = 0.85
	}

	res := Resolution{
		Resolved:  make(map[string]Candidate),
		Unknown:   []string{},
		Ambiguous: []Candidate{},
	}

	// 1. Phase 1: Check cache
	var unknownRaws []string
	if cache != nil {
		cachedMap, err := cache.GetConfirmedMappings(ctx, kind, raws)
		if err == nil && cachedMap != nil {
			for _, raw := range raws {
				if cand, found := cachedMap[raw]; found {
					cand.Source = "cached"
					res.Resolved[raw] = cand
				} else {
					unknownRaws = append(unknownRaws, raw)
				}
			}
		} else {
			unknownRaws = raws
		}
	} else {
		unknownRaws = raws
	}

	if len(unknownRaws) == 0 {
		return res, nil
	}

	// 2. Phase 2: Suggestion Source on UNKNOWN delta only (Cost Convergence)
	if source != nil {
		suggestions, err := source.Suggest(ctx, kind, unknownRaws)
		if err == nil && len(suggestions) > 0 {
			suggestedMap := make(map[string]Candidate)
			for _, s := range suggestions {
				suggestedMap[s.RawIdentifier] = s
			}

			var stillUnknown []string
			for _, raw := range unknownRaws {
				if cand, ok := suggestedMap[raw]; ok {
					if cand.Confidence >= confidenceThreshold {
						res.Resolved[raw] = cand
					} else {
						res.Ambiguous = append(res.Ambiguous, cand)
						stillUnknown = append(stillUnknown, raw)
					}
				} else {
					stillUnknown = append(stillUnknown, raw)
				}
			}
			res.Unknown = stillUnknown
			return res, nil
		}
	}

	res.Unknown = unknownRaws
	return res, nil
}
