package aiintake

// Evidence resolution ports producer.py:703-792. The model may propose a page
// and quote, but it cannot manufacture a source: a locator is emitted only
// when its page (when supplied) and quote match an adapter-owned locator, and
// coordinates are copied from the adapter, never accepted from the model.

import "strings"

// resolveLLMEvidence returns the locators the model's proposals can be
// honestly anchored to. With no adapter locators (text-only sources) it
// returns empty — a proposed quote alone is never evidence (底线 3).
func resolveLLMEvidence(rawEvidence any, available []EvidenceLocator) []EvidenceLocator {
	if len(available) == 0 {
		return nil
	}
	if m, ok := rawEvidence.(map[string]any); ok {
		for _, key := range []string{"items", "locators"} {
			if items, ok := m[key].([]any); ok {
				rawEvidence = items
				break
			}
		}
	}
	rawList, ok := rawEvidence.([]any)
	if !ok {
		return nil
	}
	var resolved []EvidenceLocator
	for _, candidateAny := range rawList {
		candidate, ok := candidateAny.(map[string]any)
		if !ok {
			continue
		}
		field := strings.TrimSpace(toString(candidate["field"]))
		quote := strings.TrimSpace(toString(candidate["quote"]))
		if field == "" || quote == "" {
			continue
		}
		page := optionalInt(candidate["page"])
		var match *EvidenceLocator
		for i := range available {
			locator := available[i]
			// Python: (page is None or locator.page == page). A proposed page
			// never matches an adapter locator without a page of its own.
			if page != nil && (locator.Page == nil || *locator.Page != *page) {
				continue
			}
			if evidenceQuoteMatches(quote, locator.Quote) {
				match = &locator
				break
			}
		}
		if match == nil {
			continue
		}
		resolved = append(resolved, EvidenceLocator{
			Field:       field,
			Source:      match.Source,
			Page:        match.Page,
			Coordinates: append([]float64(nil), match.Coordinates...),
			Quote:       quote,
		})
	}
	unique := make(map[string]EvidenceLocator, len(resolved))
	for _, locator := range resolved {
		unique[locator.Field+"|"+locator.Source+"|"+locator.Quote] = locator
	}
	out := make([]EvidenceLocator, 0, len(unique))
	for _, locator := range unique {
		out = append(out, locator)
	}
	return out
}

// evidenceCoversFields mirrors _evidence_covers_fields: every required field
// (or a dotted descendant of it) must be backed by a resolved locator.
func evidenceCoversFields(locators []EvidenceLocator, fields []string) bool {
	if len(locators) == 0 || len(fields) == 0 {
		return false
	}
	covered := make(map[string]bool, len(locators))
	for _, locator := range locators {
		if locator.Field != "" {
			covered[locator.Field] = true
		}
	}
	for _, field := range fields {
		if covered[field] {
			continue
		}
		prefix := field + "."
		ok := false
		for value := range covered {
			if strings.HasPrefix(value, prefix) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

// evidenceQuoteMatches mirrors _evidence_quote_matches: substring containment
// either way after whitespace-stripping and lowercasing. A quote the source
// never contained cannot anchor evidence.
func evidenceQuoteMatches(candidate string, source string) bool {
	left := normalizeEvidenceText(candidate)
	right := normalizeEvidenceText(source)
	if left == "" || right == "" {
		return false
	}
	return strings.Contains(left, right) || strings.Contains(right, left)
}

func normalizeEvidenceText(value string) string {
	// "".join(str(value).split()).lower() — remove every whitespace run.
	joined := strings.Join(strings.Fields(value), "")
	return strings.ToLower(joined)
}
