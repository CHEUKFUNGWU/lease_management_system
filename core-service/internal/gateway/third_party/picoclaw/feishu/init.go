// Vendored from github.com/sipeed/picoclaw at commit bbf6893ca7afad27f1d00a0f5a45982a549c6ed6.
// Upstream path: channels/feishu/init.go
// SPDX-License-Identifier: MIT — Copyright (c) 2026 PicoClaw contributors.
// See THIRD_PARTY_NOTICES at the repository root. Do not add business
// logic here; adapt upstream behaviour in the wrapping layer instead.
// Local adaptations (mechanical, behaviour-preserving):
//   - import path rewrites (picoclaw module -> this vendor tree)

package feishu

import (
	"github.com/lease-management-system/core-service/internal/gateway/third_party/picoclaw/channels"
	"github.com/lease-management-system/core-service/internal/gateway/third_party/picoclaw/config"
)

func init() {
	channels.RegisterFactory(
		config.ChannelFeishu,
		func(channelName, channelType string, cfg *config.Config, b channels.MessageBus) (channels.Channel, error) {
			bc := cfg.Channels[channelName]
			decoded, err := bc.GetDecoded()
			if err != nil {
				return nil, err
			}
			c, ok := decoded.(*config.FeishuSettings)
			if !ok {
				return nil, channels.ErrSendFailed
			}
			return NewFeishuChannel(bc, c, b)
		},
	)
}
