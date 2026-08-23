// Vendored from github.com/sipeed/picoclaw at commit bbf6893ca7afad27f1d00a0f5a45982a549c6ed6.
// Upstream path: channels/voice_capabilities.go
// SPDX-License-Identifier: MIT — Copyright (c) 2026 PicoClaw contributors.
// See THIRD_PARTY_NOTICES at the repository root. Do not add business
// logic here; adapt upstream behaviour in the wrapping layer instead.

package channels

// VoiceCapabilities describes whether ASR (speech-to-text) and TTS (text-to-speech)
// are available for a channel under the current configuration.
type VoiceCapabilities struct {
	ASR bool
	TTS bool
}

// VoiceCapabilityProvider is an optional interface for channels that want to
// explicitly declare their ASR/TTS support.
type VoiceCapabilityProvider interface {
	VoiceCapabilities() VoiceCapabilities
}

// Deprecated: Channels should implement VoiceCapabilityProvider instead.
// To be removed once all existing capable channels conform to the interface.
var asrCapableChannels = map[string]bool{
	"discord":  true,
	"telegram": true,
	"matrix":   true,
	"qq":       true,
	"weixin":   true,
	"line":     true,
	"feishu":   true,
	"onebot":   true,
}

// DetectVoiceCapabilities returns ASR/TTS availability for a channel, gated by
// whether providers are configured.
func DetectVoiceCapabilities(channelName string, ch Channel, asrAvailable bool, ttsAvailable bool) VoiceCapabilities {
	if ch == nil {
		return VoiceCapabilities{}
	}

	if vcp, ok := ch.(VoiceCapabilityProvider); ok {
		caps := vcp.VoiceCapabilities()
		if !asrAvailable {
			caps.ASR = false
		}
		if !ttsAvailable {
			caps.TTS = false
		}
		return caps
	}

	caps := VoiceCapabilities{}
	if asrAvailable {
		caps.ASR = asrCapableChannels[channelName]
	}
	if ttsAvailable {
		if _, ok := ch.(MediaSender); ok {
			caps.TTS = true
		}
	}

	return caps
}
