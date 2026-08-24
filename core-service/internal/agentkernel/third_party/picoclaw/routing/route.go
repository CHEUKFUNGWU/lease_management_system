package routing


// Vendored from github.com/sipeed/picoclaw at commit bbf6893ca7afad27f1d00a0f5a45982a549c6ed6.
// Upstream path: routing/route.go (SessionPolicy, ResolvedRoute,
// cloneIdentityLinks — symbol-level; the routing engine stays upstream).
// SPDX-License-Identifier: MIT — Copyright (c) 2026 PicoClaw contributors.

type SessionPolicy struct {
	Dimensions    []string
	IdentityLinks map[string][]string
}

type ResolvedRoute struct {
	AgentID       string
	Channel       string
	AccountID     string
	SessionPolicy SessionPolicy
	MatchedBy     string
}

func cloneIdentityLinks(src map[string][]string) map[string][]string {
	if len(src) == 0 {
		return nil
	}
	cloned := make(map[string][]string, len(src))
	for canonical, ids := range src {
		dup := make([]string, len(ids))
		copy(dup, ids)
		cloned[canonical] = dup
	}
	return cloned
}
