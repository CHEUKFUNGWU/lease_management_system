package aiagent

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/repository"
)

// errDBTX answers every query with no rows, so a real repository object
// constructs without a database yet any read degrades to a not-found error.
// That is exactly what lets the regression tell “production port wired” apart
// from “nil port (unavailable)”.
type errDBTX struct{}

func (errDBTX) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, pgx.ErrNoRows
}
func (errDBTX) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, pgx.ErrNoRows
}
func (errDBTX) QueryRow(context.Context, string, ...any) pgx.Row { return errRow{} }

type errRow struct{}

func (errRow) Scan(...any) error { return pgx.ErrNoRows }

// TestStatementModelToolsWireProductionDBAgainstNil (P0-8 / SM7): with a real
// finModelRepo the three statement-model tools must be bound to their
// production ports — a read of a non-existent run must answer a not-found
// error, never the nil-port “unavailable” stub. This is the regression that
// the unconditional nil registration used to make impossible.
func TestStatementModelToolsWireProductionPorts(t *testing.T) {
	agent := newAgent(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		repository.NewFinModelRepository(errDBTX{}), nil, nil, nil, nil)
	runtime := agent.ToolRuntime()
	if runtime == nil {
		t.Fatal("runtime must exist with a repo wired")
	}
	ctx := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{
			UserID:      "user-1",
			Permissions: []string{"reports:read"},
			Scope:       access.Scope{LegalEntityID: "le-1"},
			AgentMode:   "assist",
		},
		RunID: "run-1",
	})
	for _, tc := range []struct{ tool, args string }{
		{"fpna.statement_model.read", `{"run_id":"missing-run"}`},
		{"fpna.statement_model.evaluate", `{"model":{"definition_id":"missing-def"}}`},
	} {
		_, err := runtime.Execute(ctx, agenttools.ToolCall{
			CallID:      "call-1",
			RunID:       "run-1",
			ToolName:    tc.tool,
			ToolVersion: "v1",
			Arguments:   []byte(tc.args),
		})
		if err == nil {
			t.Fatalf("%s: a missing run/definition must error, got nil (a nil port would also error, but never 'unavailable')", tc.tool)
		}
		if strings.Contains(err.Error(), "unavailable") {
			t.Fatalf("%s: production port must be wired — got nil-port 'unavailable': %v", tc.tool, err)
		}
	}
}
