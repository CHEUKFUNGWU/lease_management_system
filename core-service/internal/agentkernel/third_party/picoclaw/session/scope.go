// Vendored from github.com/sipeed/picoclaw at commit bbf6893ca7afad27f1d00a0f5a45982a549c6ed6.
// Upstream path: session/scope.go (whole file — SessionScope is a plain data
// struct; the session engine stays upstream).
// SPDX-License-Identifier: MIT — Copyright (c) 2026 PicoClaw contributors.

package session

// ScopeVersionV1 is the first structured session-scope schema version.
const ScopeVersionV1 = 1

// SessionScope describes the semantic session partition selected for a turn.
type SessionScope struct {
	Version    int               `json:"version"`
	AgentID    string            `json:"agent_id"`
	Channel    string            `json:"channel"`
	Account    string            `json:"account"`
	Dimensions []string          `json:"dimensions"`
	Values     map[string]string `json:"values"`
}

// CloneScope returns a deep copy of scope.
func CloneScope(scope *SessionScope) *SessionScope {
	if scope == nil {
		return nil
	}
	cloned := *scope
	if len(scope.Dimensions) > 0 {
		cloned.Dimensions = append([]string(nil), scope.Dimensions...)
	}
	if len(scope.Values) > 0 {
		cloned.Values = make(map[string]string, len(scope.Values))
		for key, value := range scope.Values {
			cloned.Values[key] = value
		}
	}
	return &cloned
}
