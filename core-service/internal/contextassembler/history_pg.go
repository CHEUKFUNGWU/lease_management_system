package contextassembler

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/agentcontext"
)

// ErrScopeDenied is the ownership refusal. Like AR2's store, a cross-tenant
// read keeps its reason instead of softening into "not found" — softening
// would mask permission problems (AGENTS.md: scope_denied 不得被软化).
var ErrScopeDenied = errors.New("scope_denied: conversation belongs to another legal entity or user")

// PgHistorySource adapts ai_chat_messages to the HistorySource port. It is
// the production IO seam for Assemble; the assembler core stays pure.
//
// Ownership enforcement happens HERE at the data boundary, mirroring AR2's
// store_postgres: the session row is located by the key's session id and its
// legal entity / user are compared against the key. A foreign session (or a
// legacy NULL-entity session, which can never match a key) refuses with
// ErrScopeDenied — never an empty history that pretends nothing is there.
type PgHistorySource struct {
	pool *pgxpool.Pool
	// maxMessages bounds one read. Compaction decisions only need recent
	// turns; 500 comfortably exceeds any live conversation while keeping the
	// query bounded like every other list path in this codebase.
	maxMessages int
}

func NewPgHistorySource(pool *pgxpool.Pool) *PgHistorySource {
	return &PgHistorySource{pool: pool, maxMessages: 500}
}

// Read maps stored rows onto assembler messages, newest-bounded but returned
// in conversation order (ascending sequence).
//
// Kind is KindText for every row — a registered design fact: tool execution
// traces live in run_events, not in ai_chat_messages, so today's history has
// no audit-bearing content to protect. The taxonomy activates when tool
// messages enter history; no schema widening is done for that future now.
//
// MeasuredTokens rides from the measured_tokens column (migration 063): rows
// whose round reported provider usage count by truth, everything else falls
// to the tail estimator inside Assemble.
func (s *PgHistorySource) Read(ctx context.Context, key agentcontext.ContextKey) ([]Message, error) {
	var (
		ownerUser   string
		ownerEntity *string
	)
	err := s.pool.QueryRow(ctx, `
		SELECT user_id::text, legal_entity_id::text
		FROM ai_chat_sessions
		WHERE id = $1::uuid`,
		key.SessionID(),
	).Scan(&ownerUser, &ownerEntity)
	if errors.Is(err, pgx.ErrNoRows) {
		// Unknown locator: no conversation exists to read — an empty history,
		// not a permission conclusion. Ownership refusal is reserved for rows
		// that exist and belong to someone else.
		return []Message{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load ai chat session %s for context assembly: %w", key.SessionID(), err)
	}
	if ownerEntity == nil || *ownerEntity != key.LegalEntityID() || ownerUser != key.UserID() {
		return nil, fmt.Errorf("%w (session %s)", ErrScopeDenied, key.SessionID())
	}

	rows, err := s.pool.Query(ctx, `
		SELECT m.id::text, m.role, m.content, m.measured_tokens
		FROM (
			SELECT id, role, content, measured_tokens, sequence_no
			FROM ai_chat_messages
			WHERE session_id = $1::uuid
			ORDER BY sequence_no DESC
			LIMIT $2
		) m
		ORDER BY m.sequence_no ASC`,
		key.SessionID(), s.maxMessages,
	)
	if err != nil {
		return nil, fmt.Errorf("read ai chat messages for session %s: %w", key.SessionID(), err)
	}
	defer rows.Close()

	messages := make([]Message, 0)
	for rows.Next() {
		var (
			id      string
			role    string
			content string
			tokens  int
		)
		if err := rows.Scan(&id, &role, &content, &tokens); err != nil {
			return nil, fmt.Errorf("scan ai chat message for session %s: %w", key.SessionID(), err)
		}
		messages = append(messages, Message{
			Ref:            id,
			Role:           role,
			Kind:           KindText,
			Text:           content,
			MeasuredTokens: tokens,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ai chat messages for session %s: %w", key.SessionID(), err)
	}
	return messages, nil
}

var _ HistorySource = (*PgHistorySource)(nil)
