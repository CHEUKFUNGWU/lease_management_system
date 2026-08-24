// Vendored from github.com/sipeed/picoclaw at commit bbf6893ca7afad27f1d00a0f5a45982a549c6ed6.
// Upstream path: constants/constants.go
// SPDX-License-Identifier: MIT — Copyright (c) 2026 PicoClaw contributors.
// See THIRD_PARTY_NOTICES at the repository root. Do not add business
// logic here; adapt upstream behaviour in the wrapping layer instead.

// Package constants provides shared constants across the codebase.
package constants

// internalChannels defines channels that are used for internal communication
// and should not be exposed to external users or recorded as last active channel.
var internalChannels = map[string]struct{}{
	"cli":      {},
	"system":   {},
	"subagent": {},
}

// IsInternalChannel returns true if the channel is an internal channel.
func IsInternalChannel(channel string) bool {
	_, found := internalChannels[channel]
	return found
}
