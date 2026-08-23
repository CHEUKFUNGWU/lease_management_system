package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// ChannelIdentityBinding is one row of channel identity → internal user.
// Ch3a 租户层（ADR-0026 §3）：渠道身份经此表映射到既有内部用户，再由与 JWT
// 完全相同的 Scope 解析器产出权限；本表不携带任何权限字段——它只回答
// 「这个渠道身份是谁」，不回答「这个人能看什么」。
type ChannelIdentityBinding struct {
	ID             string
	Channel        string
	ExternalUserID string
	InternalUserID string
	BoundBy        *string
	BoundAt        string
}

// ErrChannelIdentityUnbound 是「未绑定」的具名错误：Resolve 查不到绑定时返回
// 它，绝不落到默认/匿名/兜底租户（D-B14）。
var ErrChannelIdentityUnbound = errors.New("channel identity is not bound to an internal user")

// ChannelIdentityBindingRepository owns the binding table's small SQL surface.
type ChannelIdentityBindingRepository struct {
	db DBTX
}

func NewChannelIdentityBindingRepository(db DBTX) *ChannelIdentityBindingRepository {
	return &ChannelIdentityBindingRepository{db: db}
}

func normalizeChannel(channel string) (string, error) {
	channel = strings.ToLower(strings.TrimSpace(channel))
	switch channel {
	case "feishu", "wecom":
		return channel, nil
	default:
		return "", fmt.Errorf("unsupported channel %q", channel)
	}
}

func validateChannelIdentity(channel, externalUserID string) (string, error) {
	channel, err := normalizeChannel(channel)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(externalUserID) == "" {
		return "", errors.New("external user id is required")
	}
	return channel, nil
}

// Create binds a channel identity to an internal user. Re-binding the same
// (channel, external id) pair fails on the unique constraint — 换绑走显式
// DELETE + INSERT，不静默覆盖。
func (r *ChannelIdentityBindingRepository) Create(ctx context.Context, binding *ChannelIdentityBinding) (*ChannelIdentityBinding, error) {
	channel, err := validateChannelIdentity(binding.Channel, binding.ExternalUserID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(binding.InternalUserID) == "" {
		return nil, errors.New("internal user id is required")
	}
	binding.Channel = channel
	binding.ExternalUserID = strings.TrimSpace(binding.ExternalUserID)
	var boundBy *string
	var boundAt string
	err = r.db.QueryRow(ctx, `
		INSERT INTO channel_identity_bindings (channel, external_user_id, internal_user_id, bound_by)
		VALUES ($1, $2, $3, NULLIF($4, ''))
		RETURNING id, bound_by::text, bound_at::text
	`, binding.Channel, binding.ExternalUserID, binding.InternalUserID, derefString(binding.BoundBy)).
		Scan(&binding.ID, &boundBy, &boundAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create channel identity binding: %w", err)
	}
	binding.BoundBy = boundBy
	binding.BoundAt = boundAt
	return binding, nil
}

// FindInternalUserID resolves one channel identity to its internal user id.
// 未命中返回 pgx.ErrNoRows —— 调用方（gateway.Resolve）把它翻译成具名的
// ErrChannelIdentityUnbound。
func (r *ChannelIdentityBindingRepository) FindInternalUserID(ctx context.Context, channel, externalUserID string) (string, error) {
	channel, err := validateChannelIdentity(channel, externalUserID)
	if err != nil {
		return "", err
	}
	var userID string
	err = r.db.QueryRow(ctx, `
		SELECT internal_user_id::text
		FROM channel_identity_bindings
		WHERE channel = $1 AND external_user_id = $2
	`, channel, strings.TrimSpace(externalUserID)).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrChannelIdentityUnbound
		}
		return "", fmt.Errorf("failed to find channel identity binding: %w", err)
	}
	return userID, nil
}

// Delete removes a binding (换绑/解绑走这里，审计在调用方记录).
func (r *ChannelIdentityBindingRepository) Delete(ctx context.Context, channel, externalUserID string) error {
	channel, err := validateChannelIdentity(channel, externalUserID)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx,
		`DELETE FROM channel_identity_bindings WHERE channel = $1 AND external_user_id = $2`,
		channel, strings.TrimSpace(externalUserID))
	return err
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
