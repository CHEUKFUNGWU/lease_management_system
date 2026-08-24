//go:build picoclaw_agent_core

// Vendored from github.com/sipeed/picoclaw at commit bbf6893ca7afad27f1d00a0f5a45982a549c6ed6.
// Upstream path: agent/pipeline.go
// SPDX-License-Identifier: MIT — Copyright (c) 2026 PicoClaw contributors.
// See THIRD_PARTY_NOTICES at the repository root. Do not add business
// logic here; adapt upstream behaviour in the wrapping layer instead.
// Local adaptations (mechanical, behaviour-preserving):
//   - import path rewrites

// PicoClaw - Ultra-lightweight personal AI agent

package agent

import (
	"github.com/lease-management-system/core-service/internal/agentkernel/third_party/picoclaw/interfaces"
	"github.com/lease-management-system/core-service/internal/agentkernel/third_party/picoclaw/config"
	"github.com/lease-management-system/core-service/internal/agentkernel/third_party/picoclaw/media"
	"github.com/lease-management-system/core-service/internal/agentkernel/third_party/picoclaw/providers"
)

// Pipeline holds the runtime dependencies used by Pipeline methods.
// It is constructed by runTurn via NewPipeline and passed to sub-methods
// so that the coordinator can delegate phase execution.
type Pipeline struct {
	Bus            interfaces.MessageBus
	Cfg            *config.Config
	ContextManager ContextManager
	Hooks          *HookManager
	Fallback       *providers.FallbackChain
	ChannelManager interfaces.ChannelManager
	MediaStore     media.MediaStore
	Steering       any // TODO: *Steering
	al             *AgentLoop
}

// NewPipeline creates a Pipeline from an AgentLoop instance.
func NewPipeline(al *AgentLoop) *Pipeline {
	return &Pipeline{
		Bus:            al.bus,
		Cfg:            al.GetConfig(),
		ContextManager: al.contextManager,
		Hooks:          al.hooks,
		Fallback:       al.fallback,
		ChannelManager: al.channelManager,
		MediaStore:     al.mediaStore,
		Steering:       al.steering,
		al:             al,
	}
}
