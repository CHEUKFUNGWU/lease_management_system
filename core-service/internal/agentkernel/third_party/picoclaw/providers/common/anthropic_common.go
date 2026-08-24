// Vendored from github.com/sipeed/picoclaw at commit bbf6893ca7afad27f1d00a0f5a45982a549c6ed6.
// Upstream path: providers/common/anthropic_common.go
// SPDX-License-Identifier: MIT — Copyright (c) 2026 PicoClaw contributors.

package common

import "strings"

// NormalizeBaseURL ensures the Anthropic base URL is properly formatted.
// It removes a trailing /v1 suffix if present (to avoid duplication), then
// re-appends /v1 when appendV1Suffix is true. An empty apiBase falls back to
// defaultBaseURL.
func NormalizeBaseURL(apiBase, defaultBaseURL string, appendV1Suffix bool) string {
	base := strings.TrimSpace(apiBase)
	if base == "" {
		return defaultBaseURL
	}

	base = strings.TrimRight(base, "/")
	if before, ok := strings.CutSuffix(base, "/v1"); ok {
		base = before
	}
	if base == "" {
		return defaultBaseURL
	}

	if appendV1Suffix {
		return base + "/v1"
	}
	return base
}
