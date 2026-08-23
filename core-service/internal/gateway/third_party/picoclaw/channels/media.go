// Vendored from github.com/sipeed/picoclaw at commit bbf6893ca7afad27f1d00a0f5a45982a549c6ed6.
// Upstream path: channels/media.go
// SPDX-License-Identifier: MIT — Copyright (c) 2026 PicoClaw contributors.
// See THIRD_PARTY_NOTICES at the repository root. Do not add business
// logic here; adapt upstream behaviour in the wrapping layer instead.

package channels

import (
	"context"

	"github.com/lease-management-system/core-service/internal/gateway/third_party/picoclaw/bus"
)

// MediaSender is an optional interface for channels that can send
// media attachments (images, files, audio, video).
// Manager discovers channels implementing this interface via type
// assertion and routes OutboundMediaMessage to them.
type MediaSender interface {
	SendMedia(ctx context.Context, msg bus.OutboundMediaMessage) ([]string, error)
}
