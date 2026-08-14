package access

import (
	"strings"
	"testing"
)

func TestEntityFilterScopedAndGlobalClauses(t *testing.T) {
	global := GlobalEntityFilter()
	if !global.IsGlobal() {
		t.Fatal("global filter must report IsGlobal")
	}
	clause, arg, err := global.SQLClause("legal_entity_id", 1)
	if err != nil || clause != "" || arg != nil {
		t.Fatalf("global SQLClause = %q, %v, %v; want no clause", clause, arg, err)
	}
	if _, err := global.LegalEntityID(); err == nil {
		t.Fatal("global filter must refuse LegalEntityID()")
	}

	scoped, err := EntityFilterFor("11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("EntityFilterFor: %v", err)
	}
	if scoped.IsGlobal() {
		t.Fatal("scoped filter must not report IsGlobal")
	}
	clause, arg, err = scoped.SQLClause("s.legal_entity_id", 7)
	if err != nil {
		t.Fatalf("scoped SQLClause: %v", err)
	}
	if clause != "s.legal_entity_id::text = $7" {
		t.Fatalf("scoped clause = %q, want %q", clause, "s.legal_entity_id::text = $7")
	}
	if arg != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("scoped arg = %v", arg)
	}
	id, err := scoped.LegalEntityID()
	if err != nil || id != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("scoped LegalEntityID = %q, %v", id, err)
	}
}

func TestEntityFilterForRejectsEmptyAndWhitespace(t *testing.T) {
	for _, bad := range []string{"", "   ", "\t\n"} {
		if _, err := EntityFilterFor(bad); err == nil {
			t.Fatalf("EntityFilterFor(%q) must be refused", bad)
		}
	}
	normalized, err := EntityFilterFor("  le-001  ")
	if err != nil {
		t.Fatalf("EntityFilterFor with surrounding spaces: %v", err)
	}
	id, _ := normalized.LegalEntityID()
	if id != "le-001" {
		t.Fatalf("scoped id was not trimmed: %q", id)
	}
}

func TestFromScope(t *testing.T) {
	global, err := FromScope(Scope{Global: true})
	if err != nil || !global.IsGlobal() {
		t.Fatalf("FromScope(global) = %+v, %v", global, err)
	}
	scoped, err := FromScope(Scope{LegalEntityID: "le-001"})
	if err != nil || scoped.IsGlobal() {
		t.Fatalf("FromScope(scoped) = %+v, %v", scoped, err)
	}
	if _, err := FromScope(Scope{}); err == nil {
		t.Fatal("FromScope(non-global without legal entity) must fail closed")
	}
	if _, err := FromScope(Scope{LegalEntityID: " "}); err == nil {
		t.Fatal("FromScope(whitespace legal entity) must fail closed")
	}
}

// The zero value is the configuration error this type exists to make
// unrepresentable: SQLClause must refuse it, never emit "no filtering".
func TestEntityFilterZeroValueFailsClosed(t *testing.T) {
	var filter EntityFilter
	if filter.IsGlobal() {
		t.Fatal("zero value must not be global")
	}
	clause, arg, err := filter.SQLClause("legal_entity_id", 1)
	if err == nil {
		t.Fatalf("zero-value SQLClause = %q, %v, %v; want an error", clause, arg, err)
	}
	if !strings.Contains(err.Error(), "zero-value EntityFilter") {
		t.Fatalf("zero-value error = %v", err)
	}
	if _, err := filter.LegalEntityID(); err == nil {
		t.Fatal("zero value must refuse LegalEntityID()")
	}
}
