package repository

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type transitionDB struct {
	tag      pgconn.CommandTag
	err      error
	lastSQL  string
	lastArgs []any
}

func (db *transitionDB) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	db.lastSQL = sql
	db.lastArgs = args
	return db.tag, db.err
}

func (*transitionDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	panic("unexpected Query call")
}

func (*transitionDB) QueryRow(context.Context, string, ...any) pgx.Row {
	panic("unexpected QueryRow call")
}

func TestContractWorkflowTransitionsRequireExpectedCurrentState(t *testing.T) {
	tests := []struct {
		name       string
		expected   string
		transition func(*ApprovalRepository) error
	}{
		{
			name:     "submit",
			expected: "approval_status IN ('draft', 'returned_to_editor', 'rejected')",
			transition: func(repo *ApprovalRepository) error {
				return repo.SubmitForReview(context.Background(), "contract-1", "user-1")
			},
		},
		{
			name:     "review",
			expected: "approval_status = 'submitted'",
			transition: func(repo *ApprovalRepository) error {
				return repo.Review(context.Background(), "contract-1", "user-2", true, "")
			},
		},
		{
			name:     "approve",
			expected: "approval_status = 'reviewed'",
			transition: func(repo *ApprovalRepository) error {
				return repo.Approve(context.Background(), "contract-1", "user-3")
			},
		},
		{
			name:     "reject",
			expected: "approval_status = 'reviewed'",
			transition: func(repo *ApprovalRepository) error {
				return repo.Reject(context.Background(), "contract-1", "user-3", "incomplete")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := &transitionDB{tag: pgconn.NewCommandTag("UPDATE 0")}
			err := tt.transition(NewApprovalRepository(db))
			if !errors.Is(err, ErrInvalidWorkflowTransition) {
				t.Fatalf("expected ErrInvalidWorkflowTransition, got %v", err)
			}
			if !strings.Contains(db.lastSQL, tt.expected) {
				t.Fatalf("expected transition query to contain %q, got %s", tt.expected, db.lastSQL)
			}
		})
	}
}

func TestEventWorkflowTransitionsRequireExpectedCurrentState(t *testing.T) {
	tests := []struct {
		name       string
		expected   string
		transition func(*EventRepository) error
	}{
		{
			name:     "submit",
			expected: "approval_status IN ('draft', 'returned_to_editor', 'rejected')",
			transition: func(repo *EventRepository) error {
				return repo.SubmitForReview(context.Background(), "event-1")
			},
		},
		{
			name:     "review",
			expected: "approval_status = 'submitted'",
			transition: func(repo *EventRepository) error {
				return repo.Review(context.Background(), "event-1", "user-2", true, "")
			},
		},
		{
			name:     "approve",
			expected: "approval_status = 'reviewed'",
			transition: func(repo *EventRepository) error {
				return repo.Approve(context.Background(), "event-1", "user-3", "modification")
			},
		},
		{
			name:     "reject",
			expected: "approval_status = 'reviewed'",
			transition: func(repo *EventRepository) error {
				return repo.Reject(context.Background(), "event-1", "user-3", "incomplete")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := &transitionDB{tag: pgconn.NewCommandTag("UPDATE 0")}
			err := tt.transition(NewEventRepository(db))
			if !errors.Is(err, ErrInvalidWorkflowTransition) {
				t.Fatalf("expected ErrInvalidWorkflowTransition, got %v", err)
			}
			if !strings.Contains(db.lastSQL, tt.expected) {
				t.Fatalf("expected transition query to contain %q, got %s", tt.expected, db.lastSQL)
			}
		})
	}
}

func TestWorkflowTransitionAcceptsSingleUpdatedRow(t *testing.T) {
	db := &transitionDB{tag: pgconn.NewCommandTag("UPDATE 1")}
	if err := NewApprovalRepository(db).Approve(context.Background(), "contract-1", "user-3"); err != nil {
		t.Fatalf("expected successful transition, got %v", err)
	}
}
