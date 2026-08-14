package access

import (
	"errors"
	"fmt"
	"strings"
)

// EntityFilter is the closed two-state legal-entity boundary that repository
// queries receive instead of a bare legalEntityID string. A string used ",
// for no filter" — the `($N=” OR legal_entity_id::text=$N)` escape hatch —
// cannot be expressed here: a filter is either explicitly global (an
// administrator acting across every legal entity) or explicitly scoped to one
// legal entity. Constructed values are the only valid values; the zero value
// `var f EntityFilter` is a configuration error and fails closed everywhere.
//
// Naming follows CONTEXT.md: this is the executable boundary of Legal Entity
// Access, not a "tenant permission" or "default scope".
type EntityFilter struct {
	global bool
	id     string
}

// GlobalEntityFilter spans every legal entity. It produces no legal-entity
// clause at all: the caller explicitly means "administrator, no filtering".
func GlobalEntityFilter() EntityFilter {
	return EntityFilter{global: true}
}

// EntityFilterFor builds a filter scoped to one legal entity. Empty and
// whitespace-only ids are refused: a scoped filter without an id would
// otherwise degrade into "no filtering" again.
func EntityFilterFor(id string) (EntityFilter, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return EntityFilter{}, errors.New("legal entity id is required")
	}
	return EntityFilter{id: id}, nil
}

// FromScope derives the filter from an access.Scope: a Global scope becomes a
// global filter; any other scope must carry a legal entity, otherwise the
// scope itself is misconfigured and the error is returned.
func FromScope(s Scope) (EntityFilter, error) {
	if s.Global {
		return GlobalEntityFilter(), nil
	}
	return EntityFilterFor(s.LegalEntityID)
}

// IsGlobal reports whether the filter spans every legal entity.
func (f EntityFilter) IsGlobal() bool {
	return f.global
}

// LegalEntityID returns the scoped legal-entity id. It errors for the global
// filter and for the zero value, which no constructor can produce.
func (f EntityFilter) LegalEntityID() (string, error) {
	if f.global {
		return "", errors.New("global entity filter has no legal entity id")
	}
	if f.id == "" {
		return "", errors.New("zero-value EntityFilter used: construct via EntityFilterFor or FromScope")
	}
	return f.id, nil
}

// SQLClause returns the SQL fragment that narrows a query to this filter's
// legal entity, together with its argument, for a column expression such as
// "legal_entity_id" or an aliased "s.legal_entity_id". The global filter
// yields no clause. The zero value yields an error: it must never be read as
// "no filtering".
func (f EntityFilter) SQLClause(column string, argIdx int) (string, interface{}, error) {
	if f.global {
		return "", nil, nil
	}
	id, err := f.LegalEntityID()
	if err != nil {
		return "", nil, err
	}
	return fmt.Sprintf("%s::text = $%d", column, argIdx), id, nil
}
