//go:build picoclaw_agent_core

// Vendored from github.com/sipeed/picoclaw at commit bbf6893ca7afad27f1d00a0f5a45982a549c6ed6.
// Upstream path: agent/turn_context.go
// SPDX-License-Identifier: MIT — Copyright (c) 2026 PicoClaw contributors.

package agent

import (
	"github.com/lease-management-system/core-service/internal/agentkernel/third_party/picoclaw/bus"
)

// TurnContext carries normalized turn-scoped facts that can be shared across
// events, hooks, and other runtime observers without re-parsing legacy fields.
type TurnContext struct {
	Inbound *bus.InboundContext    `json:"inbound,omitempty"`
	Route   any                    `json:"route,omitempty"` // *routing.ResolvedRoute; session/routing not vendored (AR5b)
	Scope   any                    `json:"scope,omitempty"` // *session.SessionScope; not vendored (AR5b)
}

func newTurnContext(
	inbound *bus.InboundContext,
	route *routing.ResolvedRoute,
	scope *session.SessionScope,
) *TurnContext {
	if inbound == nil && route == nil && scope == nil {
		return nil
	}
	return &TurnContext{
		Inbound: cloneInboundContext(inbound),
		Route:   cloneResolvedRoute(route),
		Scope:   session.CloneScope(scope),
	}
}

func cloneTurnContext(ctx *TurnContext) *TurnContext {
	if ctx == nil {
		return nil
	}
	cloned := *ctx
	cloned.Inbound = cloneInboundContext(ctx.Inbound)
	cloned.Route = cloneResolvedRoute(ctx.Route)
	cloned.Scope = session.CloneScope(ctx.Scope)
	return &cloned
}

func cloneInboundContext(ctx *bus.InboundContext) *bus.InboundContext {
	if ctx == nil {
		return nil
	}
	cloned := *ctx
	cloned.ReplyHandles = cloneStringMap(ctx.ReplyHandles)
	cloned.Raw = cloneStringMap(ctx.Raw)
	return &cloned
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(src))
	for k, v := range src {
		cloned[k] = v
	}
	return cloned
}

func cloneHookMeta(meta HookMeta) HookMeta {
	meta.turnContext = cloneTurnContext(meta.turnContext)
	return meta
}

func cloneResolvedRoute(route *routing.ResolvedRoute) *routing.ResolvedRoute {
	if route == nil {
		return nil
	}
	cloned := *route
	cloned.SessionPolicy = routing.SessionPolicy{
		Dimensions:    append([]string(nil), route.SessionPolicy.Dimensions...),
		IdentityLinks: cloneIdentityLinks(route.SessionPolicy.IdentityLinks),
	}
	return &cloned
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
